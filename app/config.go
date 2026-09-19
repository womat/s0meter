package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	MQTT           MQTTConfig                      `yaml:"mqtt"`           // MQTT client configuration
	DataFile       string                          `yaml:"dataFile"`       // Path to meter data YAML file
	BackupInterval time.Duration                   `yaml:"backupInterval"` // Backup interval as Go duration string (e.g. 60s)
	Meter          map[string]s0meters.MeterConfig `yaml:"meter"`          // Map of S0 meter configurations
}

// WebserverConfig holds HTTPS server settings.
type WebserverConfig struct {
	ListenHost string   `yaml:"listenHost"` // Host address for web server
	ListenPort int      `yaml:"listenPort"` // Port for web server
	ApiKey     string   `yaml:"apiKey"`     // API key for requests
	JwtSecret  string   `yaml:"jwtSecret"`  // Secret for JWT tokens
	JwtID      string   `yaml:"jwtID"`      // Unique JWT ID
	KeyFile    string   `yaml:"keyFile"`    // SSL private key file
	CertFile   string   `yaml:"certFile"`   // SSL certificate file
	BlockedIPs []string `yaml:"blockedIPs"` // Forbidden IP addresses or networks
	AllowedIPs []string `yaml:"allowedIPs"` // Allowed IP addresses or networks
}

// MQTTConfig holds MQTT client settings.
type MQTTConfig struct {
	Connection      string        `yaml:"connection"`      // Broker connection string
	PublishInterval time.Duration `yaml:"publishInterval"` // heartbeat interval as Go duration string (e.g. 60s)

	// MinPublishInterval is how often the publish loop checks for new pulses, and therefore the
	// shortest possible spacing between two messages of the same meter. It keeps a fast pulsing
	// meter from flooding the broker. Zero disables the change trigger: meters are then published
	// on the heartbeat only.
	MinPublishInterval time.Duration `yaml:"minPublishInterval"`
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
		MQTT: MQTTConfig{
			Connection:         "", // e.g. "tcp://mqtt.example.com:1883", empty means MQTT is disabled
			PublishInterval:    60 * time.Second,
			MinPublishInterval: 2 * time.Second,
		},
	}
}

// LoadConfig loads configuration from a YAML file and expands environment variables.
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

	// Replace environment variables in the YAML
	replaced := os.ExpandEnv(string(content))

	// Unmarshal YAML into the config struct
	if err = yaml.Unmarshal([]byte(replaced), cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// IsDevEnv returns true if the environment is development.
func (c *Config) IsDevEnv() bool {
	return c.Env == DevEnv
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

	for name, meter := range c.Meter {
		if err := meter.Validate(); err != nil {
			return fmt.Errorf("invalid config for meter %q: %w", name, err)
		}
	}

	if c.MQTT.PublishInterval < time.Second {
		return fmt.Errorf("publishInterval must be greater than 1s, got %v", c.MQTT.PublishInterval)
	}

	if c.MQTT.MinPublishInterval < 0 {
		return fmt.Errorf("minPublishInterval must not be negative, got %v", c.MQTT.MinPublishInterval)
	}

	if c.MQTT.MinPublishInterval > c.MQTT.PublishInterval {
		return fmt.Errorf("minPublishInterval (%v) must not exceed publishInterval (%v)",
			c.MQTT.MinPublishInterval, c.MQTT.PublishInterval)
	}

	if c.BackupInterval < time.Second {
		return fmt.Errorf("backupInterval must be greater than 1s, got %v", c.BackupInterval)
	}

	return nil
}
