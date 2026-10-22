package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/womat/golib/v2/web"
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
	LogDestination string                          `yaml:"logDestination"` // Log output: stdout | stderr | null | /path/to/logfile
	Webserver      WebserverConfig                 `yaml:"webserver"`      // Webserver configuration
	MQTT           *MQTTConfig                     `yaml:"mqtt"`           // MQTT client configuration; nil = no MQTT
	DataFile       string                          `yaml:"dataFile"`       // Path to meter data YAML file
	BackupInterval time.Duration                   `yaml:"backupInterval"` // Backup interval as Go duration string (e.g. 60s)
	Meter          map[string]s0meters.MeterConfig `yaml:"meter"`          // Map of S0 meter configurations
}

// WebserverConfig holds HTTPS server settings.
type WebserverConfig struct {
	ListenHost string      `yaml:"listenHost"` // Host address for web server
	ListenPort int         `yaml:"listenPort"` // Port for web server
	ApiKey     string      `yaml:"apiKey"`     // Removed in v7; only read to name the replacement
	TokenFile  string      `yaml:"tokenFile"`  // File with the accepted bearer tokens as SHA-256 hashes
	Tokens     []web.Token `yaml:"-"`          // Read from TokenFile by LoadConfig
	KeyFile    string      `yaml:"keyFile"`    // SSL private key file
	CertFile   string      `yaml:"certFile"`   // SSL certificate file
	BlockedIPs []string    `yaml:"blockedIPs"` // Forbidden IP addresses or networks
	AllowedIPs []string    `yaml:"allowedIPs"` // Allowed IP addresses or networks
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
		Env:            ProdEnv, // dev only when set explicitly: it falls back to the public dev certificate
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
// empty string when it is unset. Unlike os.ExpandEnv it leaves every other "$" alone, so a
// password containing "$" is not silently cut short.
func expandEnvBraces(s string) string {
	return envBraces.ReplaceAllStringFunc(s, func(ref string) string {
		return os.Getenv(envBraces.FindStringSubmatch(ref)[1])
	})
}

// LoadConfig loads configuration from a YAML file and expands ${VAR} environment references,
// then reads the bearer tokens from webserver.tokenFile.
//
// Unknown keys are an error rather than ignored, so a misspelled or renamed key cannot silently
// leave its setting at the default. A token file that is missing or broken is an error too: the
// service would otherwise start and reject every request with 401. Since a SIGHUP reload goes
// through here, a token added with "s0meter token add" applies on reload.
func LoadConfig(fileName string) (*Config, error) {
	cfg, err := readConfig(fileName)
	if err != nil {
		return cfg, err
	}

	if cfg.Webserver.TokenFile != "" {
		if cfg.Webserver.Tokens, err = web.ReadTokenFile(cfg.Webserver.TokenFile); err != nil {
			return cfg, fmt.Errorf("token file: %w", err)
		}
	}

	return cfg, nil
}

// TokenFile returns webserver.tokenFile of the config file, without reading the token file
// itself - "s0meter token add" has to work before that file exists.
func TokenFile(fileName string) (string, error) {
	cfg, err := readConfig(fileName)
	if err != nil {
		return "", err
	}
	return cfg.Webserver.TokenFile, nil
}

// readConfig loads the config file alone. A relative tokenFile is resolved against the
// directory of the config file, so the service and "s0meter token" find the same file wherever
// they are started from.
func readConfig(fileName string) (*Config, error) {
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

	if f := cfg.Webserver.TokenFile; f != "" && !filepath.IsAbs(f) {
		cfg.Webserver.TokenFile = filepath.Join(filepath.Dir(fileName), f)
	}

	return cfg, nil
}

// Validate checks the Config for invalid or missing values.
func (c *Config) Validate() error {

	if c.Env != ProdEnv && c.Env != DevEnv {
		return fmt.Errorf("invalid environment: %s, must be %s or %s", c.Env, ProdEnv, DevEnv)
	}

	if c.Webserver.ApiKey != "" {
		return errors.New(`webserver.apiKey is no longer supported; create a token with ` +
			`"s0meter token add <name>" and set webserver.tokenFile`)
	}
	if c.Webserver.TokenFile == "" {
		return errors.New(`webserver.tokenFile is not configured; create a token with "s0meter token add <name>"`)
	}

	// Matched like xlog.Init matches it, case-insensitively and ignoring surrounding blanks.
	validLogLevels := []string{"debug", "info", "warning", "warn", "error"}
	if !slices.Contains(validLogLevels, strings.ToLower(strings.TrimSpace(c.LogLevel))) {
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

	// The broker URL is redacted before it is logged or shown on /health, which only works when
	// url.Parse finds the user part. An unencoded "/", "#", "?" or space in the password hides
	// it, and the password would appear in plain text. The errors do not repeat the URL.
	u, err := url.Parse(m.Connection)
	if err != nil || (u.User == nil && strings.Contains(m.Connection, "@")) {
		return errors.New("mqtt.connection is not a valid URL; URL-encode special characters " +
			"in user and password, e.g. / as %2F, # as %23, ? as %3F")
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
