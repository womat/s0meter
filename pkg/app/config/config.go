package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration. Attention!
// To make it possible to overwrite fields with the -overwrite command
// line option, each of the struct fields must be in the format
// first letter uppercase -> followed by CamelCase as in the config file.
// Config defines the struct of global config and the struct of the configuration file
type Config struct {
	Flag                      FlagConfig    `yaml:"-"`
	DataCollectionInterval    time.Duration `yaml:"-"`
	DataCollectionIntervalInt int           `yaml:"datacollectioninterval"`
	DataFile                  string        `yaml:"datafile"`
	BackupInterval            time.Duration `yaml:"-"`
	BackupIntervalInt         int           `yaml:"backupinterval"`
	//	Debug                     slog.Logger            `yaml:"-"`
	Meter     map[string]MeterConfig `yaml:"meter"`
	Webserver WebserverConfig        `yaml:"webserver"`
	MQTT      MQTTConfig             `yaml:"mqtt"`
}

// FlagConfig defines the configured flags (parameters)
type FlagConfig struct {
	Version bool
	//	Debug      bool
	ConfigFile string
}

// WebserverConfig defines the struct of the webserver and webservice configuration and configuration file
type WebserverConfig struct {
	URL string `yaml:"url"`
}

// MQTTConfig defines the struct of the mqtt client configuration and configuration file
type MQTTConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Connection string `yaml:"connection"`
	Retained   bool   `yaml:"retained"`
}

// MeterConfig defines the struct of the meter configuration and configuration file
type MeterConfig struct {
	Gpio            int           `yaml:"gpio"`
	BounceTimeInt   int           `yaml:"bouncetime"`
	BounceTime      time.Duration `yaml:"-"`
	CounterConstant float64       `yaml:"counterconstant"`
	UnitCounter     string        `yaml:"unitcounter"`
	ScaleFactor     float64       `yaml:"scalefactor"`
	Precision       int           `yaml:"precision "`
	UnitGauge       string        `yaml:"unitgauge"`
	MqttTopic       string        `yaml:"mqtttopic"`
}

func NewConfig() *Config {
	return &Config{
		Flag:                      FlagConfig{},
		DataCollectionInterval:    0,
		DataCollectionIntervalInt: 0,
		DataFile:                  "/opt/womat/data/measurement.yaml",
		BackupInterval:            0,
		BackupIntervalInt:         0,

		Meter: map[string]MeterConfig{},
		Webserver: WebserverConfig{
			URL: "http://0.0.0.0:4000",
		},
		MQTT: MQTTConfig{Connection: "tcp:127.0.0.1883"},
	}
}

func (c *Config) LoadConfig() error {
	if err := c.readConfigFile(); err != nil {
		return fmt.Errorf("error reading config file %q: %w", c.Flag.ConfigFile, err)
	}

	c.DataCollectionInterval = time.Duration(c.DataCollectionIntervalInt) * time.Second
	c.BackupInterval = time.Duration(c.BackupIntervalInt) * time.Second

	for name, meter := range c.Meter {
		meter.BounceTime = time.Duration(meter.BounceTimeInt) * time.Millisecond
		c.Meter[name] = meter
	}

	return nil
}

func (c *Config) readConfigFile() error {
	file, err := os.Open(c.Flag.ConfigFile)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	decoder := yaml.NewDecoder(file)
	if err = decoder.Decode(c); err != nil {
		return err
	}

	return nil
}
