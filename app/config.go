package app

import (
	"fmt"
	"os"
	"path/filepath"
	"s0counter/app/service/s0meters"

	"gopkg.in/yaml.v3"
)

const (
	ProdEnv = "prod"
	DevEnv  = "dev"
)

// Config holds the main application configuration.
type Config struct {
	Env                    string                          `yaml:"env"`                    // Application environment: dev | prod
	LogLevel               string                          `yaml:"logLevel"`               // Log level: debug | info | warning | error
	LogDestination         string                          `yaml:"logDestination"`         // Log output: stdout | stderr | /path/to/logfile
	HttpsServer            WebserverConfig                 `yaml:"webserver"`              // Webserver configuration
	MQTT                   MQTTConfig                      `yaml:"mqtt"`                   // MQTT client configuration
	DataCollectionInterval int                             `yaml:"dataCollectionInterval"` // Meter data collection interval in seconds
	DataFile               string                          `yaml:"dataFile"`               // Path to meter data YAML file
	BackupInterval         int                             `yaml:"backupInterval"`         // Backup interval in seconds
	Meter                  map[string]s0meters.MeterConfig `yaml:"meter"`                  // Map of S0 meter configurations
}

// WebserverConfig holds HTTPS server settings.
type WebserverConfig struct {
	ListenHost string   `yaml:"listenHost"` // Host address for web server
	ListenPort string   `yaml:"listenPort"` // Port for web server
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
	Enabled    bool   `yaml:"enabled"`    // Enable or disable MQTT
	Connection string `yaml:"connection"` // Broker connection string
	Retained   bool   `yaml:"retained"`   // Whether messages are retained
}

// NewConfig returns a Config with sane defaults
func NewConfig() *Config {
	return &Config{
		Env:                    DevEnv,
		LogLevel:               "info",
		LogDestination:         "stdout",
		DataCollectionInterval: 10,
		BackupInterval:         60,
		Meter:                  make(map[string]s0meters.MeterConfig),
		HttpsServer: WebserverConfig{
			ListenHost: "0.0.0.0",
			ListenPort: "8443",
			BlockedIPs: []string{},
			AllowedIPs: []string{},
		},
		MQTT: MQTTConfig{
			Enabled: false,
		},
	}
}

// LoadConfig loads configuration from a YAML file and expands environment variables.
func (c *Config) LoadConfig(fileName string) (*Config, error) {
	fileName = filepath.ToSlash(fileName)

	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return c, fmt.Errorf("failed to read config file %s: %w", fileName, err)
	}
	if fileInfo.IsDir() {
		return c, fmt.Errorf("config path %s is a directory, not a file", fileName)
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return c, fmt.Errorf("failed to read config file %s: %w", fileName, err)
	}

	// Replace environment variables in the YAML
	replaced := os.ExpandEnv(string(content))

	// Unmarshal YAML into the config struct
	if err = yaml.Unmarshal([]byte(replaced), c); err != nil {
		return c, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return c, nil
}

// IsDevEnv returns true if the environment is development.
func (c *Config) IsDevEnv() bool {
	return c.Env == DevEnv
}
