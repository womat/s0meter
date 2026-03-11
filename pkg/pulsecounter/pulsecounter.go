// Package pulsecounter provides counting of GPIO pulses (S0 signals) on a Raspberry Pi.
//
// This package handles the low-level pulse detection from a GPIO pin, applies
// debouncing to reduce noise, and maintains a counter with timestamps of the
// last two pulses.
//
// It's intended to be used as a building block for higher-level S0 energy meters,
// which can manage multiple pulse counters or add metadata and business logic.
//
// Features:
// - Counts S0 pulses from a GPIO pin
// - Applies debouncing to filter out noise
// - Maintains timestamps of the last and penultimate pulses
// - Provides safe concurrent access with an internal mutex
// - Supports real GPIO (`rpi/gpio`) and emulated GPIO (`rpi/gpioemu`)
package pulsecounter

import (
	"context"
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
}

// New initializes a GPIO pin for S0 pulses and returns a Handler.
//
// The returned Handler is fully initialized and ready to use.
// The GPIO pin is configured as input, debounced, and events are watched.
// Returns an error if initialization fails.
func New(ctx context.Context, port int, debounce time.Duration) (*Handler, error) {

	p, err := rpi.NewPin(port,
		rpi.WithMode(gpio.Input),
		rpi.WithPullup(gpio.PullUp),
		rpi.WithDebounce(debounce))
	if err != nil {
		return nil, fmt.Errorf("create GPIO port %d: %w", port, err)
	}

	h := &Handler{pin: port, gpioPin: p}

	if err = p.WatchFunc(gpio.RisingEdge, h.handlePulseEvent); err != nil {
		p.Close()
		return nil, fmt.Errorf("start watching events on GPIO port %d: %w", port, err)
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

// SetCounter sets the pulse counter to a specific value.
func (h *Handler) SetCounter(s Counter) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.counter = s
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
	h.mu.Unlock()

	slog.Debug("s0 pulse",
		"gpio", h.pin,
		"tick", snapshot.Pulses,
		"lastTimestamp", snapshot.LastTimeStamp,
		"currentTimestamp", snapshot.TimeStamp,
	)
}
