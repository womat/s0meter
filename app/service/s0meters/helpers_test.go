package s0meters

import (
	"testing"

	"github.com/womat/golib/gpio"
	"github.com/womat/golib/gpio/rpiemu"
	"github.com/womat/s0meter/pkg/pulsecounter"
)

// newMeter returns a meter on an emulated GPIO pin that starts at pulses.
func newMeter(t *testing.T, cfg MeterConfig, pulses uint64) *MeterInstance {
	t.Helper()

	pin, err := rpiemu.NewPin(cfg.Gpio, rpiemu.WithMode(gpio.Input))
	if err != nil {
		t.Fatal(err)
	}
	meter, err := pulsecounter.NewWithPin(pin, pulses)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = meter.Close() })

	return &MeterInstance{Config: cfg, Meter: meter}
}

// handlerWith returns a Handler holding the given meters.
func handlerWith(meters map[string]*MeterInstance) *Handler {
	h := New()
	h.meters = meters
	return h
}
