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

func TestMissedPulsesAreCountedAndSkipTheGaugeInterval(t *testing.T) {
	h, _ := newEmulated(t, 10)
	t0 := time.Now()

	h.handlePulseEvent(gpio.Event{Time: t0, Edge: gpio.RisingEdge})
	h.handlePulseEvent(gpio.Event{Time: t0.Add(time.Second), Edge: gpio.RisingEdge, Missed: 3})

	c := h.GetCounter()
	if c.Pulses != 15 {
		t.Errorf("pulses = %d, want 15 (10 + 1 + 3 missed + 1)", c.Pulses)
	}
	if !c.LastTimeStamp.IsZero() {
		t.Errorf("LastTimeStamp = %v after a gap, want zero", c.LastTimeStamp)
	}

	h.handlePulseEvent(gpio.Event{Time: t0.Add(2 * time.Second), Edge: gpio.RisingEdge})
	c = h.GetCounter()
	if c.Pulses != 16 {
		t.Errorf("pulses = %d, want 16", c.Pulses)
	}
	if got := c.TimeStamp.Sub(c.LastTimeStamp); got != time.Second {
		t.Errorf("interval after the gap = %v, want 1s", got)
	}
}

// TestRecountsPulsesDroppedWhileProcessingFellBehind overflows the GPIO event buffer by
// stalling the handler, and checks that the dropped pulses still end up in the counter.
func TestRecountsPulsesDroppedWhileProcessingFellBehind(t *testing.T) {
	h, pin := newEmulated(t, 0)

	// The first event reaches handlePulseEvent and waits for the lock, the next ones fill
	// the buffer, and the rest are dropped.
	const driven = 100
	h.mu.Lock()
	for range driven {
		pulse(t, pin)
	}
	h.mu.Unlock()

	dropped := h.DroppedEvents()
	if dropped == 0 {
		t.Fatal("no event was dropped; the test did not overflow the buffer")
	}
	// Let the buffer drain, so the next pulse is delivered and reports the gap.
	waitForPulses(t, h, driven-dropped)

	pulse(t, pin)
	c := waitForPulses(t, h, driven+1)
	if !c.LastTimeStamp.IsZero() {
		t.Errorf("LastTimeStamp = %v after a gap, want zero", c.LastTimeStamp)
	}
}
