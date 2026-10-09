package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/womat/s0meter/app/service/s0meters"
	"gopkg.in/yaml.v3"
)

const (
	ProdEnv = "prod"
	DevEnv  = "dev"
)

// Config holds the main application configuration.
type Config struct {
	Env            string                          `yaml:"env"`            // Application environment: dev | prod
	LogLevel       string                          `yaml:"logLevel"`       // Log level: debug | info | warning | error
	LogDestination string                          `yaml:"logDestination"` // Log output: stdout | stderr | /path/to/logfile
	Webserver      WebserverConfig                 `yaml:"webserver"`      // Webserver configuration
	MQTT           *MQTTConfig                     `yaml:"mqtt"`           // MQTT client configuration; nil = no MQTT
	DataFile       string                          `yaml:"dataFile"`       // Path to meter data YAML file
	BackupInterval time.Duration                   `yaml:"backupInterval"` // Backup interval as Go duration string (e.g. 60s)
	Meter          map[string]s0meters.MeterConfig `yaml:"meter"`          // Map of S0 meter configurations
}

// WebserverConfig holds HTTPS server settings.
type WebserverConfig struct {
	ListenHost string   `yaml:"listenHost"` // Host address for web server
	ListenPort int      `yaml:"listenPort"` // Port for web server
	ApiKey     string   `yaml:"apiKey"`     // API key for requests
	KeyFile    string   `yaml:"keyFile"`    // SSL private key file
	CertFile   string   `yaml:"certFile"`   // SSL certificate file
	BlockedIPs []string `yaml:"blockedIPs"` // Forbidden IP addresses or networks
	AllowedIPs []string `yaml:"allowedIPs"` // Allowed IP addresses or networks
}

// MQTTConfig holds MQTT client settings. MQTT is on when the mqtt block is present.
type MQTTConfig struct {
	Connection      string        `yaml:"connection"`      // Broker connection string, required
	PublishInterval time.Duration `yaml:"publishInterval"` // heartbeat interval as Go duration string (default 60s)

	// MinPublishInterval is how often the publish loop checks for new pulses, and therefore the
	// shortest possible spacing between two messages of the same meter. It keeps a fast pulsing
	// meter from flooding the broker. nil = 2s; 0 disables the change trigger: meters are then
	// published on the heartbeat only.
	MinPublishInterval *time.Duration `yaml:"minPublishInterval"`
}

// Default MQTT intervals, applied by Validate to a present mqtt block.
const (
	defaultPublishInterval    = 60 * time.Second
	defaultMinPublishInterval = 2 * time.Second
)

// MinInterval returns MinPublishInterval, or its default when it is not set.
func (m *MQTTConfig) MinInterval() time.Duration {
	if m.MinPublishInterval == nil {
		return defaultMinPublishInterval
	}
	return *m.MinPublishInterval
}

// NewConfig returns a Config with sane defaults
func NewConfig() *Config {
	return &Config{
		Env:            DevEnv,
		LogLevel:       "info",
		LogDestination: "stdout",
		BackupInterval: 60 * time.Second,
		Meter:          make(map[string]s0meters.MeterConfig),
		DataFile:       filepath.Join("/opt", MODULE, "data", "s0meter.yaml"),
		Webserver: WebserverConfig{
			ListenHost: "0.0.0.0",
			ListenPort: 8443,
			BlockedIPs: []string{},
			AllowedIPs: []string{},
		},
	}
}

// envBraces matches ${VAR} references; see expandEnvBraces.
var envBraces = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvBraces replaces ${VAR} with the value of the environment variable VAR, or with an
// empty string when it is unset. Unlike os.ExpandEnv it leaves every other "$" alone, so an API
// key or password containing "$" is not silently cut short.
func expandEnvBraces(s string) string {
	return envBraces.ReplaceAllStringFunc(s, func(ref string) string {
		return os.Getenv(envBraces.FindStringSubmatch(ref)[1])
	})
}

// LoadConfig loads configuration from a YAML file and expands ${VAR} environment references.
//
// Unknown keys are an error rather than ignored, so a misspelled or renamed key cannot silently
// leave its setting at the default.
func LoadConfig(fileName string) (*Config, error) {
	cfg := NewConfig()

	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return cfg, err
	}
	if fileInfo.IsDir() {
		return cfg, errors.New("config path is a directory, not a file")
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return cfg, err
	}

	dec := yaml.NewDecoder(bytes.NewReader([]byte(expandEnvBraces(string(content)))))
	dec.KnownFields(true)
	if err = dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Validate checks the Config for invalid or missing values.
func (c *Config) Validate() error {

	if c.Env != ProdEnv && c.Env != DevEnv {
		return fmt.Errorf("invalid environment: %s, must be %s or %s", c.Env, ProdEnv, DevEnv)
	}

	if c.Webserver.ApiKey == "" {
		return errors.New("ApiKey is not configured")
	}

	validLogLevels := []string{"debug", "info", "warning", "warn", "error"}
	if !slices.Contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("invalid log level: %s, must be one of %v", c.LogLevel, validLogLevels)
	}

	if c.Webserver.ListenPort < 1 || c.Webserver.ListenPort > 65535 {
		return fmt.Errorf("invalid port: %d", c.Webserver.ListenPort)
	}

	// Visit the meters in a fixed order, so the reported duplicate does not depend on map order.
	names := make([]string, 0, len(c.Meter))
	for name := range c.Meter {
		names = append(names, name)
	}
	sort.Strings(names)

	gpioUsedBy := make(map[int]string, len(names))
	for _, name := range names {
		meter := c.Meter[name]
		if err := meter.Validate(); err != nil {
			return fmt.Errorf("invalid config for meter %q: %w", name, err)
		}
		if other, used := gpioUsedBy[meter.Gpio]; used {
			return fmt.Errorf("meters %q and %q both use gpio %d", other, name, meter.Gpio)
		}
		gpioUsedBy[meter.Gpio] = name
	}

	if err := c.MQTT.validate(); err != nil {
		return err
	}

	if c.BackupInterval < time.Second {
		return fmt.Errorf("backupInterval must be at least 1s, got %v", c.BackupInterval)
	}

	return nil
}

// validate applies the defaults of a present mqtt block and checks it; a nil block (no MQTT)
// is valid.
func (m *MQTTConfig) validate() error {
	if m == nil {
		return nil
	}

	if m.Connection == "" {
		return errors.New("mqtt.connection is required; delete the mqtt block to run without MQTT")
	}

	if m.PublishInterval == 0 {
		m.PublishInterval = defaultPublishInterval
	}
	if m.PublishInterval < time.Second {
		return fmt.Errorf("publishInterval must be at least 1s, got %v", m.PublishInterval)
	}

	if m.MinInterval() < 0 {
		return fmt.Errorf("minPublishInterval must not be negative, got %v", m.MinInterval())
	}

	if m.MinInterval() > m.PublishInterval {
		return fmt.Errorf("minPublishInterval (%v) must not exceed publishInterval (%v)",
			m.MinInterval(), m.PublishInterval)
	}

	return nil
}

// minApiKeyLength is the length below which Warnings flags the API key as weak.
const minApiKeyLength = 16

// Warnings returns findings that do not stop the service but should be fixed. It never includes
// secret values.
func (c *Config) Warnings() []string {
	var warnings []string

	key := c.Webserver.ApiKey
	switch {
	case strings.Contains(strings.ToLower(key), "changeme"):
		warnings = append(warnings, "apiKey is still the example value from the documentation; set a random key")
	case len(key) < minApiKeyLength:
		warnings = append(warnings, fmt.Sprintf("apiKey is shorter than %d characters; use a longer random key", minApiKeyLength))
	}

	return warnings
}
