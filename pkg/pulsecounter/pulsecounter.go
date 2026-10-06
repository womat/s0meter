// Package pulsecounter counts S0 pulses (DIN 43864) on one GPIO line.
//
// It watches the rising edge of a debounced input with pull-up and keeps the pulse count
// together with the timestamps of the last two pulses. It knows nothing of units or
// scaling; that is left to the caller.
//
// New opens a Raspberry Pi GPIO line through golib's gpio/rpi. NewWithPin takes any
// gpio.Pin instead, which is how the tests drive it with golib's in-memory gpio/rpiemu.
// A Handler is safe for concurrent use.
package pulsecounter

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/womat/golib/gpio"
	"github.com/womat/golib/gpio/rpi"
)

// Counter stores S0 pulse data.
type Counter struct {
	Pulses        uint64    `yaml:"pulses"`        // total number of pulses
	TimeStamp     time.Time `yaml:"timestamp"`     // timestamp of last pulse
	LastTimeStamp time.Time `yaml:"lastTimestamp"` // timestamp of penultimate pulse
}

// Handler manages a GPIO pin and counts S0 pulses.
type Handler struct {
	mu      sync.Mutex
	counter Counter
	gpioPin gpio.Pin
	pin     int
	dropped uint64 // DroppedEvents of the pin at Close, reported once the pin is gone
}

// New opens GPIO port as a debounced input with pull-up and counts its pulses.
//
// Counting continues from pulses, which is set before watching starts, so no pulse
// counted in the meantime can be overwritten by restoring a saved value later.
// Returns an error if initialization fails.
func New(port int, debounce time.Duration, pulses uint64) (*Handler, error) {
	p, err := rpi.NewPin(port,
		rpi.WithMode(gpio.Input),
		rpi.WithPullup(gpio.PullUp),
		rpi.WithDebounce(debounce))
	if err != nil {
		return nil, fmt.Errorf("create GPIO port %d: %w", port, err)
	}

	h, err := NewWithPin(p, pulses)
	if err != nil {
		_ = p.Close()
		return nil, err
	}
	return h, nil
}

// NewWithPin counts the rising edges of an already configured pin, continuing from pulses.
// The Handler takes ownership of the pin and closes it in Close.
func NewWithPin(p gpio.Pin, pulses uint64) (*Handler, error) {
	h := &Handler{pin: p.Number(), gpioPin: p, counter: Counter{Pulses: pulses}}

	if err := p.WatchFunc(gpio.RisingEdge, h.handlePulseEvent); err != nil {
		return nil, fmt.Errorf("start watching events on GPIO port %d: %w", h.pin, err)
	}

	return h, nil
}

// GetCounter returns a snapshot of the current counter.
// Safe for concurrent access. Returns count of pulse and timestamps of the last two pulses.
func (h *Handler) GetCounter() Counter {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.counter
}

// DroppedEvents returns how many edge events were lost since the pin was opened, because
// pulse processing fell behind or the kernel's event buffer overflowed. Each one is a pulse
// that was counted late, with the next event that got through; only the gauge interval
// across the gap is lost. The count starts at zero with every New, so it resets on a
// configuration reload.
func (h *Handler) DroppedEvents() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.gpioPin == nil {
		return h.dropped
	}
	return h.gpioPin.DroppedEvents()
}

// Close stops event watching and releases the GPIO pin.
// Safe to call multiple times. Returns any errors from StopWatching and Close.
func (h *Handler) Close() error {
	h.mu.Lock()
	p := h.gpioPin
	h.gpioPin = nil
	if p != nil {
		h.dropped = p.DroppedEvents()
	}
	h.mu.Unlock()

	if p == nil {
		return nil
	}

	err1 := p.StopWatching()
	err2 := p.Close()

	return errors.Join(err1, err2)
}

// handlePulseEvent is triggered on GPIO events.
// Updates the counter and timestamps in a thread-safe manner.
//
// e.Missed counts the rising edges lost right before this one. The line is debounced in
// the kernel and watched on the rising edge only, so each of them is a real pulse and is
// added to the counter. The interval to the previous pulse then spans more than one pulse,
// so LastTimeStamp is cleared and the gauge restarts with the next pulse.
func (h *Handler) handlePulseEvent(e gpio.Event) {
	h.mu.Lock()
	if h.gpioPin == nil {
		h.mu.Unlock()
		return
	}

	h.counter.LastTimeStamp = h.counter.TimeStamp
	if e.Missed > 0 {
		h.counter.LastTimeStamp = time.Time{}
	}
	h.counter.TimeStamp = e.Time
	h.counter.Pulses += 1 + e.Missed
	snapshot := h.counter
	h.mu.Unlock()

	if e.Missed > 0 {
		slog.Warn("s0 pulses lost before this one, counted late; the gauge skips this interval",
			"gpio", h.pin,
			"missed", e.Missed,
			"pulses", snapshot.Pulses,
		)
	}

	slog.Debug("s0 pulse",
		"gpio", h.pin,
		"tick", snapshot.Pulses,
		"lastTimestamp", snapshot.LastTimeStamp,
		"currentTimestamp", snapshot.TimeStamp,
	)
}
