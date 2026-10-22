package app

import (
	"net/url"
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
	// readConfig: the example names /opt/s0meter/etc/token-hashes, which does not exist here.
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
	c.Webserver.TokenFile = "/opt/s0meter/etc/token-hashes"
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

	// Each case names a part of the message it must produce, so it cannot pass for another
	// reason when validConfig changes.
	invalid := map[string]struct {
		mutate func(*Config)
		want   string
	}{
		"missing tokenFile": {func(c *Config) { c.Webserver.TokenFile = "" }, "tokenFile is not configured"},
		"unknown env":       {func(c *Config) { c.Env = "staging" }, "invalid environment"},
		"unknown log level": {func(c *Config) { c.LogLevel = "verbose" }, "invalid log level"},
		"port 0":            {func(c *Config) { c.Webserver.ListenPort = 0 }, "invalid port"},
		"port 65536":        {func(c *Config) { c.Webserver.ListenPort = 65536 }, "invalid port"},
		"backup below 1s":   {func(c *Config) { c.BackupInterval = 500 * time.Millisecond }, "backupInterval must be at least 1s"},
		"invalid meter":     {func(c *Config) { c.Meter["a"] = meterOn(1) }, `meter "a"`},
		"duplicate gpio": {func(c *Config) { c.Meter["a"] = meterOn(17); c.Meter["b"] = meterOn(17) },
			`meters "a" and "b" both use gpio 17`},
		"display unit of another quantity": {func(c *Config) {
			m := meterOn(2)
			m.CounterUnit, m.DisplayUnit = "Wh", "m³"
			c.Meter["a"] = m
		}, "displayUnit"},
		"unknown display gauge unit": {func(c *Config) {
			m := meterOn(2)
			m.GaugeUnit, m.DisplayGaugeUnit = "W", "PS"
			c.Meter["a"] = m
		}, "displayGaugeUnit"},
		"publishInterval below 1s": {func(c *Config) {
			c.MQTT = &MQTTConfig{Connection: "tcp://broker:1883", PublishInterval: 500 * time.Millisecond}
		}, "publishInterval must be at least 1s"},
		"negative minPublishInterval": {func(c *Config) {
			minInterval := -time.Second
			c.MQTT = &MQTTConfig{Connection: "tcp://broker:1883", MinPublishInterval: &minInterval}
		}, "must not be negative"},
		"minPublishInterval above publishInterval": {func(c *Config) {
			minInterval := 2 * time.Minute
			c.MQTT = &MQTTConfig{Connection: "tcp://broker:1883", MinPublishInterval: &minInterval}
		}, "must not exceed publishInterval"},
	}
	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestMQTTBlockPresenceAndDefaults(t *testing.T) {
	base := "webserver:\n  tokenFile: /opt/s0meter/etc/token-hashes\n"
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

// A broker URL whose password is not URL-encoded cannot be redacted: url.Parse no longer finds
// the user part, so the password would reach the log and /health in plain text (S2). Such a
// URL is refused, and the error does not repeat it.
func TestValidateRefusesBrokerURLWithUnencodedPassword(t *testing.T) {
	const secret = "s3cr"
	for _, connection := range []string{
		"tcp://user:123/" + secret + "@broker:1883", // digits + "/": parses, but without a user
		"tcp://user:" + secret + "#1@broker:1883",
		"tcp://user:" + secret + "?1@broker:1883",
		"tcp://user:" + secret + " 1@broker:1883",
	} {
		c := NewConfig()
		c.Webserver.TokenFile = "token-hashes"
		c.MQTT = &MQTTConfig{Connection: connection}
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "URL-encode") {
			t.Errorf("Validate(%q) = %v, want an error asking to URL-encode", connection, err)
			continue
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("the error for %q repeats the password: %v", connection, err)
		}
	}

	for _, connection := range []string{
		"tcp://broker:1883",
		"tcp://user:" + url.QueryEscape("123/s3cr#?") + "@broker:1883",
	} {
		c := NewConfig()
		c.Webserver.TokenFile = "token-hashes"
		c.MQTT = &MQTTConfig{Connection: connection}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want it accepted", connection, err)
		}
	}
}
