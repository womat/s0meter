package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	cfg, err := LoadConfig("../config/config.yaml")
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
	cfg, err := LoadConfig(writeConfig(t, "webserver:\n  apiKey: ${S0METER_TEST_KEY}\nmqtt:\n  connection: tcp://u:pa$word@broker:1883\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Webserver.ApiKey != "from-env" {
		t.Errorf("apiKey = %q, want from-env", cfg.Webserver.ApiKey)
	}
	if cfg.MQTT.Connection != "tcp://u:pa$word@broker:1883" {
		t.Errorf("a bare $ was changed: %q", cfg.MQTT.Connection)
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
	c.Webserver.ApiKey = "0123456789abcdefXYZ"
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
		"missing apiKey": func(c *Config) { c.Webserver.ApiKey = "" },
		"unknown env":    func(c *Config) { c.Env = "staging" },
		"invalid meter":  func(c *Config) { c.Meter["a"] = meterOn(1) },
		"duplicate gpio": func(c *Config) { c.Meter["a"] = meterOn(17); c.Meter["b"] = meterOn(17) },
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

func TestWarnings(t *testing.T) {
	c := validConfig()
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("strong key: unexpected warnings %v", w)
	}

	for _, key := range []string{"Xq7z", "changeme!", "my-ChangeMe-key-that-is-long"} {
		c.Webserver.ApiKey = key
		w := c.Warnings()
		if len(w) != 1 {
			t.Errorf("key %q: warnings = %v, want one", key, w)
			continue
		}
		if strings.Contains(w[0], key) {
			t.Errorf("warning leaks the key: %q", w[0])
		}
	}
}
