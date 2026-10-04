package pulsecounter

import (
	"testing"
	"time"

	"github.com/womat/golib/gpio"
	"github.com/womat/golib/gpio/rpiemu"
)

// newEmulated returns a Handler on an emulated input pin, plus the pin to drive it.
func newEmulated(t *testing.T, pulses uint64) (*Handler, rpiemu.Pin) {
	t.Helper()

	pin, err := rpiemu.NewPin(17, rpiemu.WithMode(gpio.Input), rpiemu.WithPullup(gpio.PullUp))
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewWithPin(pin, pulses)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h, pin
}

// pulse drives one complete S0 pulse: low, then the rising edge that is counted.
func pulse(t *testing.T, pin rpiemu.Pin) {
	t.Helper()
	if err := pin.Drive(gpio.Low); err != nil {
		t.Fatal(err)
	}
	if err := pin.Drive(gpio.High); err != nil {
		t.Fatal(err)
	}
}

// waitForPulses waits until the asynchronously delivered events have been counted.
func waitForPulses(t *testing.T, h *Handler, want uint64) Counter {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		c := h.GetCounter()
		if c.Pulses == want {
			return c
		}
		if time.Now().After(deadline) {
			t.Fatalf("pulses = %d, want %d", c.Pulses, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCountsRisingEdgesFromStartValue(t *testing.T) {
	h, pin := newEmulated(t, 100)

	if got := h.GetCounter().Pulses; got != 100 {
		t.Fatalf("start value = %d, want 100", got)
	}

	before := time.Now()
	for range 3 {
		pulse(t, pin)
	}
	c := waitForPulses(t, h, 103)

	if c.TimeStamp.Before(before) || c.LastTimeStamp.Before(before) {
		t.Errorf("timestamps %v / %v lie before the pulses", c.LastTimeStamp, c.TimeStamp)
	}
	if !c.TimeStamp.After(c.LastTimeStamp) {
		t.Errorf("TimeStamp %v is not after LastTimeStamp %v", c.TimeStamp, c.LastTimeStamp)
	}
	if d := h.DroppedEvents(); d != 0 {
		t.Errorf("DroppedEvents = %d, want 0", d)
	}
}

func TestStopsCountingAfterClose(t *testing.T) {
	h, pin := newEmulated(t, 0)

	pulse(t, pin)
	waitForPulses(t, h, 1)

	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}

	pulse(t, pin)
	time.Sleep(20 * time.Millisecond)
	if got := h.GetCounter().Pulses; got != 1 {
		t.Errorf("pulses after Close = %d, want 1", got)
	}
}
