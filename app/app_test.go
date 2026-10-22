package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/womat/s0meter/app/service/s0meters"
)

func TestRedactURL(t *testing.T) {
	got := redactURL("tcp://user:secret@mqtt.example.com:1883")
	if strings.Contains(got, "secret") || !strings.Contains(got, "mqtt.example.com") {
		t.Errorf("redactURL = %q, want the host without the password", got)
	}
}

// A Run that fails while registering the meters must not save: the data file would lose the
// counters of every meter not registered yet.
func TestFailedRunKeepsDataFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "s0meter.yaml")
	saved := "wallbox:\n    pulses: 4711\n"
	if err := os.WriteFile(file, []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := NewConfig()
	cfg.DataFile = file
	// No GPIO chip offers line 999, so registering this meter fails.
	cfg.Meter = map[string]s0meters.MeterConfig{
		"wallbox": {Gpio: 999, CounterPulsesPerUnit: 1, GaugeScale: 1},
	}

	// The error must come from the registration: a data file wrongly refused while reading
	// would leave it unchanged too.
	if _, err := New(cfg, make(chan os.Signal), nil).Run(); err == nil || !strings.Contains(err.Error(), "GPIO port 999") {
		t.Fatalf("Run() = %v, want the GPIO registration to fail", err)
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != saved {
		t.Errorf("data file after a failed Run = %q, %v; want it unchanged", got, err)
	}
}

// A damaged data file is marked ErrPermanent, so cmd exits with 78; a busy GPIO line may free
// itself and is not.
func TestRunMarksDataFileErrorsPermanent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "s0meter.yaml")
	if err := os.WriteFile(file, []byte("wallbox:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.DataFile = file
	if _, err := New(cfg, make(chan os.Signal), nil).Run(); !errors.Is(err, ErrPermanent) {
		t.Errorf("Run() with a damaged data file = %v, want ErrPermanent", err)
	}

	cfg.DataFile = filepath.Join(t.TempDir(), "s0meter.yaml")
	cfg.Meter = map[string]s0meters.MeterConfig{"wallbox": {Gpio: 999, CounterPulsesPerUnit: 1, GaugeScale: 1}}
	if _, err := New(cfg, make(chan os.Signal), nil).Run(); err == nil || errors.Is(err, ErrPermanent) {
		t.Errorf("Run() with a GPIO line that cannot be opened = %v, want an error that is not ErrPermanent", err)
	}
}
