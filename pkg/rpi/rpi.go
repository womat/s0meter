// Package rpi defines a generic interface for GPIO event handling on Raspberry Pi.
//
// This package provides a standard GPIO interface that can be implemented by different backends.
// It defines event types for rising and falling edges, an Event struct with timestamps, and
// a common interface (`GPIO`) for interacting with GPIO lines.
//
// Features:
// - Defines GPIO event types (`LineEventRisingEdge` and `LineEventFallingEdge`)
// - Provides an Event struct containing a timestamp and event type
// - Defines a GPIO interface (`GPIO`) that different implementations can use
// - Supports input/output modes, pull-up/pull-down resistors, and debounce settings
// - Can be implemented with various hardware backends (e.g., `gpioemu`, `gpiod`)
//
// Example Usage with different backends:
//
//	// Using a real GPIO backend
//	import "github.com/example/gpiodriver"
//
//	gpioDevice, err := gpiodriver.NewPort(17)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Using an emulated GPIO backend
//	import "github.com/example/gpioemu"
//
//	emulatedDevice, err := gpioemu.NewPort(17, time.Millisecond*100)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// General usage with any backend implementing `rpi.GPIO`
//	var gpioPort rpi.GPIO = gpioDevice
//
//	gpioPort.SetOutputMode()
//	gpioPort.SetValue(1)
//
//	gpioPort.StartWatchingEvents(func(evt rpi.Event) {
//	    fmt.Println("GPIO Event:", evt)
//	})
//
// Note: This package does not interact with hardware directly but defines
// the interface for GPIO event handling, allowing multiple implementations.
package rpi

import (
	"time"
)

const (
	// LineEventFallingEdge indicates an active to inactive event (high to low).
	LineEventFallingEdge int = iota

	// LineEventRisingEdge indicates an inactive event to an active event (low to high).
	LineEventRisingEdge
)

// Event represents a state change event on a line.
type Event struct {
	// Timestamp is the exact time when the event was detected.
	Timestamp time.Time
	// Type represents the type of state change event (LineEventRisingEdge or LineEventFallingEdge).
	Type int
}

type GPIO interface {
	Close() error
	GetValue() (int, error)
	Port() int
	Info() string
	SetDebounceTime(time.Duration) error
	SetInputMode() error
	SetOutputMode() error
	SetPullDown() error
	SetPullUp() error
	SetValue(int) error
	StartWatchingEvents(func(Event)) error
	StopWatchingEvents() error
}
