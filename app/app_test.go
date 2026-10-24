package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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

// With an mqtt block Run starts although the broker cannot be reached: the client keeps trying
// in the background, /health reports it as disconnected, and the stop disconnects it, so the
// next App does not run a second client with the same ID. It takes the 5 s womat/mqtt waits for
// the first connect.
func TestRunWithUnreachableBrokerStartsAndDisconnects(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig()
	cfg.DataFile = filepath.Join(dir, "s0meter.yaml")
	cfg.Webserver.ListenHost = "127.0.0.1"
	cfg.Webserver.ListenPort = 0
	cfg.Env = DevEnv
	cfg.MQTT = &MQTTConfig{Connection: "tcp://127.0.0.1:1", PublishInterval: time.Minute}

	signals := make(chan os.Signal, 1)
	app, err := New(cfg, signals, nil).Run()
	if err != nil {
		t.Fatal(err)
	}
	if app.mqtt == nil || !app.mqtt.IsConnected() {
		t.Fatal("Run did not leave an MQTT client trying to connect")
	}
	if got := app.mqttState(); got != mqttDisconnected {
		t.Errorf("mqttState = %q, want %q", got, mqttDisconnected)
	}

	signals <- syscall.SIGTERM
	select {
	case <-app.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("SIGTERM did not stop the App")
	}
	if app.mqtt.IsConnected() {
		t.Error("the stopped App left its MQTT client connecting")
	}
}
