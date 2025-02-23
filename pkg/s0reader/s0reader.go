// Package s0reader provides functionality for counting S0 pulses from a Raspberry Pi GPIO port.
//
// S0 pulses are commonly used in energy meters to measure power consumption. This package
// initializes a GPIO port to read these pulses, applies debouncing, and maintains a counter.
//
// Features:
// - Reads S0 pulses from a GPIO port
// - Uses debouncing to filter out noise
// - Keeps track of timestamps for the last and penultimate pulses
// - Provides safe concurrent access via mutex protection
// - Supports both real GPIO (`rpi/gpio`) and emulated GPIO (`rpi/gpioemu`)
//
// Example Usage:
//
//	meter := s0reader.New()
//
//	err := meter.InitializePort(17, time.Millisecond*100)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	defer meter.Close()
//
//	counter := meter.GetCounter()
//	fmt.Println("S0 Pulse Count:", counter.Tick)
//
// Note: This package requires a Raspberry Pi with access to GPIO ports.
package s0reader

import (
	"github.com/womat/golib/rpi"
	"github.com/womat/golib/rpi/gpio"
	"log/slog"
	"sync"
	"time"
)

// Counter is the struct to store the s0 pulse data
type Counter struct {
	Tick          uint64    `yaml:"ticks"`         // s0 ticks overall
	TimeStamp     time.Time `yaml:"timestamp"`     // time of the last s0 pulse
	LastTimeStamp time.Time `yaml:"lastTimestamp"` // time of the penultimate s0 pulse
}

// Handler is the struct to store the meter data
type Handler struct {
	mux      sync.RWMutex
	counter  Counter
	gpioPort rpi.GPIO
}

// New initializes and returns a new S0 meter handler.
//
// The returned `Handler` manages an internal counter and can be used to track
// S0 pulses from a GPIO port. The GPIO port is not initialized automatically;
// `InitializePort()` must be called before usage.
//
// Example:
//
//	meter := s0reader.New()
func New() *Handler {
	return &Handler{
		mux:     sync.RWMutex{},
		counter: Counter{},
	}
}

// GetCounter returns the current S0 pulse counter.
//
// This function provides safe concurrent access to the counter-value by
// using a read lock. It returns a `Counter` struct containing the total
// number of ticks and timestamps of the last two pulses.
//
// Example:
//
//	counter := meter.GetCounter()
//	fmt.Println("Total Pulses:", counter.Tick)
func (h *Handler) GetCounter() (c Counter) {
	h.mux.RLock()
	c = h.counter
	h.mux.RUnlock()
	return c
}

// SetCounter sets the S0 pulse counter to a specific value.
//
// This function safely modifies the counter by acquiring a write lock.
// It allows manual resetting or modification of the counter-values.
//
// Example:
//
//	meter.SetCounter(s0meter.Counter{Tick: 100})
func (h *Handler) SetCounter(s Counter) {
	h.mux.Lock()
	h.counter = s
	h.mux.Unlock()
}

// InitializePort initializes the GPIO port for detecting S0 pulses.
//
// This function configures the GPIO port as an input, applies debouncing,
// and registers an event handler to track rising-edge pulses.
//
// Parameters:
// - `port`: The GPIO pin number where the S0 signal is connected.
// - `debounce`: The debounce duration to filter out noise in the signal.
//
// Returns an error if the port cannot be initialized.
//
// Example:
//
//	err := meter.InitializePort(17, time.Millisecond*100)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (h *Handler) InitializePort(port int, debounce time.Duration) (err error) {

	if h.gpioPort, err = gpio.NewPort(port); err != nil {
		//if h.gpioPort, err = gpioemu.NewPort(port, time.Second); err != nil {
		return err
	}
	if err = h.gpioPort.SetInputMode(); err != nil {
		return err
	}
	if err = h.gpioPort.SetDebounceTime(debounce); err != nil {
		return err
	}
	if err = h.gpioPort.WatchingEvents(h.handlePulseEvent); err != nil {
		return err
	}

	return nil
}

// Close releases the GPIO port and stops event watching.
//
// This function ensures that no further pulse events are processed
// and safely closes the GPIO port.
//
// Example:
//
//	defer meter.Close()
func (h *Handler) Close() error {
	if h.gpioPort == nil {
		return nil
	}
	_ = h.gpioPort.StopWatching()
	return h.gpioPort.Close()
}

// handlePulseEvent processes an S0 pulse event detected on the GPIO port.
//
// This function is automatically triggered when a rising-edge event occurs.
// It updates the pulse counter and timestamps while ensuring thread safety.
//
// Example (internal usage):
//
//	meter.handlePulseEvent(rpi.Event{Timestamp: time.Now(), Type: rpi.LineEventRisingEdge})
func (h *Handler) handlePulseEvent(e rpi.Event) {
	port := h.gpioPort.Port()
	slog.Debug("s0 pulse detected", "gpio", port, "edge", e.Edge, "eventTime", e.Time)

	if e.Edge == rpi.RisingEdge {
		h.mux.Lock()
		h.counter.LastTimeStamp = h.counter.TimeStamp
		h.counter.TimeStamp = e.Time
		h.counter.Tick++
		tmp := h.counter // copy for logging
		h.mux.Unlock()
		slog.Debug("Rising Edge detected", "gpio", port, "tick", tmp.Tick, "lastTimestamp", tmp.LastTimeStamp, "currentTimestamp", tmp.TimeStamp)
	}
}
