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
	dropped uint64 // dropped events already reported by handlePulseEvent
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

// DroppedEvents returns how many edge events the GPIO layer dropped since the pin was
// opened because pulse processing fell behind. Each one is a pulse missing from the counter.
// The count starts at zero with every New, so it resets on a configuration reload.
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
func (h *Handler) handlePulseEvent(e gpio.Event) {
	h.mu.Lock()
	if h.gpioPin == nil {
		h.mu.Unlock()
		return
	}

	h.counter.LastTimeStamp = h.counter.TimeStamp
	h.counter.TimeStamp = e.Time
	h.counter.Pulses++
	snapshot := h.counter

	// Events dropped while the buffer was full surface here, with the next event
	// that did get through.
	dropped := h.gpioPin.DroppedEvents()
	newlyDropped := dropped - h.dropped
	h.dropped = dropped
	h.mu.Unlock()

	if newlyDropped > 0 {
		slog.Warn("s0 pulses lost, GPIO events were dropped because pulse processing fell behind",
			"gpio", h.pin,
			"dropped", newlyDropped,
			"droppedTotal", dropped,
		)
	}

	slog.Debug("s0 pulse",
		"gpio", h.pin,
		"tick", snapshot.Pulses,
		"lastTimestamp", snapshot.LastTimeStamp,
		"currentTimestamp", snapshot.TimeStamp,
	)
}
