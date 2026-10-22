package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/womat/golib/v2/web"
	"github.com/womat/gpio"
	"github.com/womat/gpio/rpiemu"
	"github.com/womat/s0meter/app/service/s0meters"
	"github.com/womat/s0meter/pkg/pulsecounter"
)

// emulatedPins opens meters on emulated GPIO pins and remembers each pin and the start value it
// was opened with. The open number failAt fails, counting from 1; 0 never fails.
type emulatedPins struct {
	failAt int

	mu     sync.Mutex
	opened int
	pins   map[int]*closeRecorder
	starts map[int]uint64
}

// closeRecorder is an emulated pin that remembers being closed.
type closeRecorder struct {
	rpiemu.Pin
	closed atomic.Bool
}

func (p *closeRecorder) Close() error {
	p.closed.Store(true)
	return p.Pin.Close()
}

func (e *emulatedPins) open(line int, debounce time.Duration, pulses uint64) (*pulsecounter.Handler, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.opened++
	if e.opened == e.failAt {
		return nil, errors.New("GPIO line busy")
	}
	p, err := rpiemu.NewPin(line, rpiemu.WithMode(gpio.Input), rpiemu.WithDebounce(debounce))
	if err != nil {
		return nil, err
	}
	pin := &closeRecorder{Pin: p}
	if e.pins == nil {
		e.pins, e.starts = map[int]*closeRecorder{}, map[int]uint64{}
	}
	e.pins[line], e.starts[line] = pin, pulses
	return pulsecounter.NewWithPin(pin, pulses)
}

func (e *emulatedPins) pin(line int) *closeRecorder {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pins[line]
}

// newMeterApp returns an App whose meters count on emulated pins, with the data file saved and
// the token "test-key".
func newMeterApp(t *testing.T, saved string, meters map[string]s0meters.MeterConfig) (*App, *emulatedPins) {
	t.Helper()
	cfg := NewConfig()
	cfg.DataFile = filepath.Join(t.TempDir(), "s0meter.yaml")
	if saved != "" {
		if err := os.WriteFile(cfg.DataFile, []byte(saved), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Meter = meters
	cfg.Webserver.Tokens = []web.Token{{Name: "test", Hash: web.HashToken("test-key")}}

	pins := &emulatedPins{}
	app := New(cfg, make(chan os.Signal), nil)
	app.meters = s0meters.NewWith(pins.open)
	return app, pins
}

// pulse drives one S0 pulse on pin.
func pulse(t *testing.T, pin rpiemu.Pin) {
	t.Helper()
	if err := pin.Drive(gpio.Low); err != nil {
		t.Fatal(err)
	}
	if err := pin.Drive(gpio.High); err != nil {
		t.Fatal(err)
	}
}

// waitForCounter waits until the asynchronously counted pulses reach want.
func waitForCounter(t *testing.T, app *App, meter string, want float64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		m, err := app.meters.GetMeter(meter)
		if err == nil && m.Counter == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s counter = %v (%v), want %v", meter, m.Counter, err, want)
		}
		time.Sleep(time.Millisecond)
	}
}

// Init opens every meter with its saved value, so no pulse counted during start-up can be
// overwritten by the restore; Cleanup closes the pins before the final save, so the file holds
// every counted pulse and nothing after it.
func TestInitRestoresAndCleanupSavesLastPulse(t *testing.T) {
	app, pins := newMeterApp(t, "wallbox:\n    pulses: 4711\n", map[string]s0meters.MeterConfig{
		"wallbox": {Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1},
	})

	if err := app.Init(); err != nil {
		t.Fatal(err)
	}
	if got := pins.starts[17]; got != 4711 {
		t.Fatalf("pin opened with %d pulses, want the saved 4711", got)
	}

	pin := pins.pin(17)
	pulse(t, pin)
	waitForCounter(t, app, "wallbox", 4712)

	if err := app.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !pin.closed.Load() {
		t.Error("Cleanup left the GPIO pin open")
	}

	// The pin no longer delivers events, so this pulse is neither counted nor saved.
	pulse(t, pin)
	if m, _ := app.meters.GetMeter("wallbox"); m.Counter != 4712 {
		t.Errorf("counter = %v after Cleanup, want 4712", m.Counter)
	}
	saved, err := s0meters.ReadMeterData(app.config.DataFile)
	if err != nil || saved["wallbox"] != 4712 {
		t.Errorf("data file = %v, %v; want wallbox at 4712", saved, err)
	}
}

// A Run that fails on the second meter closes the pin of the first, so the next App can open
// it again, and leaves the data file alone.
func TestFailedRegistrationClosesOpenedPins(t *testing.T) {
	saved := "wallbox:\n    pulses: 4711\n"
	app, pins := newMeterApp(t, saved, map[string]s0meters.MeterConfig{
		"wallbox": {Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1},
		"water":   {Gpio: 5, CounterPulsesPerUnit: 1, GaugeScale: 1},
	})
	pins.failAt = 2

	if _, err := app.Run(); err == nil {
		t.Fatal("Run() succeeded although the second meter failed to open")
	}

	if len(pins.pins) != 1 {
		t.Fatalf("%d pins opened, want 1", len(pins.pins))
	}
	for line, pin := range pins.pins {
		if !pin.closed.Load() {
			t.Errorf("GPIO %d still open after the failed Run", line)
		}
	}
	if got, err := os.ReadFile(app.config.DataFile); err != nil || string(got) != saved {
		t.Errorf("data file after a failed Run = %q, %v; want it unchanged", got, err)
	}
}

// /meters and /meters/{name} return the reading of a registered meter.
func TestMetersEndpointsServeRegisteredMeter(t *testing.T) {
	app, _ := newMeterApp(t, "wallbox:\n    pulses: 4711\n", map[string]s0meters.MeterConfig{
		"wallbox": {Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, CounterUnit: "Wh", GaugeUnit: "W"},
	})
	if err := app.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Cleanup() })

	rec := serve(app, "/meters", "test-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /meters = %d, want 200", rec.Code)
	}
	var all map[string]s0meters.MeterData
	if err := json.Unmarshal(rec.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if m := all["wallbox"]; len(all) != 1 || m.Counter != 4711 || m.CounterUnit != "Wh" {
		t.Errorf("GET /meters = %s", rec.Body)
	}

	rec = serve(app, "/meters/wallbox", "test-key")
	var one s0meters.MeterData
	if err := json.Unmarshal(rec.Body.Bytes(), &one); rec.Code != http.StatusOK || err != nil || one.Meter != "wallbox" || one.Counter != 4711 {
		t.Errorf("GET /meters/wallbox = %d %s", rec.Code, rec.Body)
	}

	if rec := serve(app, "/meters/unknown", "test-key"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /meters/unknown = %d, want 404", rec.Code)
	}
}

// A saved counter whose meter is no longer configured stays in the data file until someone
// removes it there, so commenting a meter out and back in does not lose its count (F3).
func TestUnconfiguredCounterStaysInDataFile(t *testing.T) {
	app, _ := newMeterApp(t, "wallbox:\n    pulses: 4711\nheatpump:\n    pulses: 99\n", map[string]s0meters.MeterConfig{
		"wallbox": {Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1},
	})
	if err := app.Init(); err != nil {
		t.Fatal(err)
	}

	saved, err := s0meters.ReadMeterData(app.config.DataFile)
	if err != nil || saved["heatpump"] != 99 {
		t.Errorf("data file after Init = %v, %v; want heatpump kept at 99", saved, err)
	}

	if err := app.Cleanup(); err != nil {
		t.Fatal(err)
	}
	saved, err = s0meters.ReadMeterData(app.config.DataFile)
	if err != nil || saved["heatpump"] != 99 || saved["wallbox"] != 4711 {
		t.Errorf("data file after Cleanup = %v, %v; want heatpump 99 and wallbox 4711", saved, err)
	}
	if _, err := app.meters.GetMeter("heatpump"); err == nil {
		t.Error("the unconfigured meter is served by the API")
	}
}
