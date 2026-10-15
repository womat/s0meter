package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/womat/s0meter/app/service/s0meters"
)

// writeConfig writes content to a temporary config file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestLoadConfigExample(t *testing.T) {
	// readConfig: the example names /opt/s0meter/etc/tokens, which does not exist here.
	cfg, err := readConfig("../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the shipped example config does not validate: %v", err)
	}
	if len(cfg.Meter) == 0 {
		t.Error("the example config defines no meters")
	}
}

func TestLoadConfigRejectsUnknownKeys(t *testing.T) {
	for name, content := range map[string]string{
		"renamed meter key": "meter:\n  m:\n    gpio: 5\n    bounceTime: 1\n",
		"removed jwtSecret": "webserver:\n  jwtSecret: x\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, content)); err == nil {
				t.Error("expected an error for the unknown key")
			}
		})
	}
}

func TestLoadConfigEmptyFileGivesDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Webserver.ListenPort != NewConfig().Webserver.ListenPort {
		t.Errorf("listenPort = %d, want the default", cfg.Webserver.ListenPort)
	}
}

func TestLoadConfigExpandsBracedVariablesOnly(t *testing.T) {
	t.Setenv("S0METER_TEST_KEY", "from-env")
	cfg, err := LoadConfig(writeConfig(t, "mqtt:\n  connection: tcp://u:${S0METER_TEST_KEY}@broker:1883/pa$word\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.Connection != "tcp://u:from-env@broker:1883/pa$word" {
		t.Errorf("connection = %q, want the variable expanded and the bare $ kept", cfg.MQTT.Connection)
	}
}

func TestExpandEnvBraces(t *testing.T) {
	t.Setenv("S0METER_TEST_KEY", "secret")
	for in, want := range map[string]string{
		"${S0METER_TEST_KEY}":   "secret",
		"a${S0METER_TEST_KEY}b": "asecretb",
		"$S0METER_TEST_KEY":     "$S0METER_TEST_KEY",
		"ab$cd":                 "ab$cd",
		"${S0METER_UNSET_X1}":   "",
		"$$x${":                 "$$x${",
	} {
		if got := expandEnvBraces(in); got != want {
			t.Errorf("expandEnvBraces(%q) = %q, want %q", in, got, want)
		}
	}
}

// validConfig returns a config that passes Validate.
func validConfig() *Config {
	c := NewConfig()
	c.Webserver.TokenFile = "/opt/s0meter/etc/tokens"
	return c
}

func meterOn(gpio int) s0meters.MeterConfig {
	return s0meters.MeterConfig{Gpio: gpio, CounterPulsesPerUnit: 1, GaugeScale: 1}
}

func TestValidate(t *testing.T) {
	c := validConfig()
	c.Meter["a"] = meterOn(2)
	c.Meter["b"] = meterOn(27)
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}

	invalid := map[string]func(*Config){
		"missing tokenFile": func(c *Config) { c.Webserver.TokenFile = "" },
		"unknown env":       func(c *Config) { c.Env = "staging" },
		"invalid meter":     func(c *Config) { c.Meter["a"] = meterOn(1) },
		"duplicate gpio":    func(c *Config) { c.Meter["a"] = meterOn(17); c.Meter["b"] = meterOn(17) },
		"display unit of another quantity": func(c *Config) {
			m := meterOn(2)
			m.CounterUnit, m.DisplayUnit = "Wh", "m³"
			c.Meter["a"] = m
		},
		"unknown display gauge unit": func(c *Config) {
			m := meterOn(2)
			m.GaugeUnit, m.DisplayGaugeUnit = "W", "PS"
			c.Meter["a"] = m
		},
	}
	for name, mutate := range invalid {
		t.Run(name, func(t *testing.T) {
			c := validConfig()
			mutate(c)
			if err := c.Validate(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestMQTTBlockPresenceAndDefaults(t *testing.T) {
	base := "webserver:\n  tokenFile: /opt/s0meter/etc/tokens\n"
	load := func(t *testing.T, content string) (*Config, error) {
		t.Helper()
		cfg, err := readConfig(writeConfig(t, base+content))
		if err != nil {
			t.Fatal(err)
		}
		return cfg, cfg.Validate()
	}

	cfg, err := load(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT != nil {
		t.Errorf("mqtt = %+v without an mqtt block, want nil (no MQTT)", cfg.MQTT)
	}

	cfg, err = load(t, "mqtt:\n  connection: tcp://broker:1883\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.PublishInterval != 60*time.Second || cfg.MQTT.MinInterval() != 2*time.Second {
		t.Errorf("intervals = %v / %v, want the defaults 60s / 2s", cfg.MQTT.PublishInterval, cfg.MQTT.MinInterval())
	}

	// 0s is a setting of its own: publish on the heartbeat only.
	cfg, err = load(t, "mqtt:\n  connection: tcp://broker:1883\n  minPublishInterval: 0s\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.MinInterval() != 0 {
		t.Errorf("minPublishInterval 0s = %v, want 0 (change trigger off)", cfg.MQTT.MinInterval())
	}

	// A present block must be complete: an empty connection used to mean "MQTT off".
	for _, content := range []string{"mqtt:\n  connection: \"\"\n", "mqtt:\n  publishInterval: 30s\n"} {
		if _, err = load(t, content); err == nil || !strings.Contains(err.Error(), "delete the mqtt block") {
			t.Errorf("Validate() = %v for %q, want the hint to delete the mqtt block", err, content)
		}
	}
}
