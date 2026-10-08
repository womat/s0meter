package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(file)
}

// yaml.v3 refuses a duration without a unit, 0 included; this pins that, so a bare number can
// never be read as nanoseconds.
func TestLoadConfigRejectsDurationWithoutUnit(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, key string }{
		"backupInterval": {"backupInterval: 60\n", "backupInterval"},
		"meter debounce": {"meter:\n  wallbox:\n    gpio: 5\n    debounceTime: 10\n", "meter.wallbox.debounceTime"},
		"minPublish":     {"mqtt:\n  minPublishInterval: 2\n", "mqtt.minPublishInterval"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadYAML(t, tc.yaml)
			if err == nil || !strings.Contains(err.Error(), "time.Duration") {
				t.Errorf("%s: err = %v, want a refused duration", tc.key, err)
			}
		})
	}
}

func TestLoadConfigAcceptsDurationsWithUnit(t *testing.T) {
	if _, err := loadYAML(t, "backupInterval: 1m\nmqtt:\n  minPublishInterval: 0s\nmeter:\n  wallbox:\n    gpio: 5\n    debounceTime: 10ms\n"); err != nil {
		t.Errorf("valid durations refused: %v", err)
	}
}
