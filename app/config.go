package app

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"s0counter/app/service/s0meters"
)

const (
	ProdEnv = "prod"
	DevEnv  = "dev"
)

// Config holds the application configuration
type Config struct {
	// Env is the app environment.
	// Env is read from APP_ENV environment variable.
	//  Allowed values: prod | dev
	//  It's used for:
	//  - jwt token expiration (1 day in dev, 5 minutes in prod)
	Env string

	// LogLevel is the log level, if set only message with at least this level is logged
	//  e.g.: debug -> means error, warning, info and debug messages are logged
	// Allowed values: debug | info | warning | error
	LogLevel string `yaml:"logLevel"`

	// LogDestination defines the log destinations.
	//  supported values: stdout | stderr | /path/to/logfile
	LogDestination string `yaml:"logDestination"`

	// HttpsServer is the configuration of the webserver and webservice
	HttpsServer WebserverConfig `yaml:"webserver"`
	MQTT        MQTTConfig      `yaml:"mqtt"`

	DataCollectionInterval int                             `yaml:"dataCollectionInterval"`
	DataFile               string                          `yaml:"dataFile"`
	BackupInterval         int                             `yaml:"backupInterval"`
	Meter                  map[string]s0meters.MeterConfig `yaml:"meter"`
}

// WebserverConfig defines the struct of the webserver and webservice configuration and configuration file
type WebserverConfig struct {
	// ListenHost is the host address the https server listens for connections.
	ListenHost string `yaml:"listenHost"`

	// ListenPort is the port the https server listens for connections.
	ListenPort string `yaml:"listenPort"`

	// ApiKey is the global api key for the application.
	ApiKey string `yaml:"apiKey"`

	// JwtSecret is a secret key used to sign jwt tokens.
	JwtSecret string `yaml:"jwtSecret"`

	// JwtID is a unique identifier for the jwt token used to prevent login with the same jwt token to another app.
	JwtID string `yaml:"jwtID"`

	// KeyFile is the ssl certificate private key file
	KeyFile string `yaml:"keyFile"`

	// CertFile is the ssl certificate public key file
	// Pfx files are supported as well, in which case KeyFile must be empty and CertFile must point to the pfx file, CertPassword must contain the password to decode the pfx file.
	CertFile string `yaml:"certFile"`

	// BlockedIPs is a list of IP addresses or networks that are forbidden from accessing the application.
	// Default is empty, which means no IP addresses or networks are blocked.
	// Multiple IP addresses or networks can be defined separated by a comma
	// e.g.: 192.168.0.1,192.168.0.0/16,10.0.0.0/8,192.168.254.15
	BlockedIPs []string `yaml:"blockedIPs"`

	// AllowedIPs is a list of IP addresses that are allowed to access the application.
	// Default is empty, which means all IP addresses are allowed.
	// The value "ALL" allows access from all IP Addresses / IP Networks
	// multiple IP addresses or networks can be defined separated by a comma
	// e.g.: 127.0.0.1,::1,192.168.0.0/16,10.0.0.0/8
	// Note: '::1' is the IPv6 loopback address.
	AllowedIPs []string `yaml:"allowedIPs"`
}

// MQTTConfig defines the struct of the mqtt client configuration and configuration file
type MQTTConfig struct {
	Enabled    bool   `yaml:"-"`
	Connection string `yaml:"connection"`
	Retained   bool   `yaml:"retained"`
}

// NewConfig initializes and returns a new Config.
func NewConfig() *Config {
	return &Config{
		Meter: make(map[string]s0meters.MeterConfig),
		HttpsServer: WebserverConfig{
			BlockedIPs: []string{},
			AllowedIPs: []string{},
		},
	}
}

// LoadConfig loads a configuration file into the Config struct.
// It reads the file, expands environment variables, and unmarshals the YAML content into the struct.
func (c *Config) LoadConfig(fileName string) (*Config, error) {

	fileName = filepath.ToSlash(fileName)

	if fileInfo, err := os.Stat(fileName); err != nil || fileInfo.IsDir() {
		return nil, fmt.Errorf("invalid or missing file %s", fileName)
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return c, err
	}

	replaced := os.ExpandEnv(string(content))
	err = yaml.Unmarshal([]byte(replaced), c)
	return c, err
}

// IsDevEnv returns true if "dev" is configured as app environment.
func (c *Config) IsDevEnv() bool {
	return c.Env == DevEnv
}
