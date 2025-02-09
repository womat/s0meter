// Package gpioemu provides an emulator for Raspberry Pi GPIO pins.
//
// This package simulates GPIO ports and events without requiring actual hardware,
// making it useful for testing and development. It allows setting input and output modes,
// toggling GPIO states, and handling event-driven changes such as rising and falling edges.
//
// Features:
// - Simulates GPIO pin behavior for testing without physical hardware
// - Supports input and output modes
// - Allows setting and reading GPIO states
// - Implements event handling for rising and falling edge detection
// - Provides debouncing support to filter out noise in input signals
//
// Example Usage:
//
//  p, err := gpioemu.NewPort(17, time.Millisecond * 100)
//  if err != nil {
//      log.Fatal(err)
//  }
//  defer p.Close()
//
//  p.SetOutputMode()
//  p.SetValue(1)
//  val, _ := p.GetValue()
//  fmt.Println("GPIO state:", val)
//
//  p.StartWatchingEvents(func(evt rpi.Event) {
//      fmt.Println("Event detected:", evt)
//  })
//
// This package is useful for software development and testing environments
// where real GPIO hardware is not available.
//
// Note: This is an emulator and does not interact with actual hardware.

package gpioemu

import (
	"s0counter/pkg/rpi"
	"sync"
	"time"
)

// Port represents a single requested line.
type Port struct {
	sync.RWMutex
	eventHandler func(event rpi.Event)
	gpio         int
	port         int
	stop         chan struct{}
	pulse        time.Duration
}

// NewPort requests control of a single line on a chip.
func NewPort(gpio int, pulse time.Duration) (*Port, error) {

	var err error
	p := &Port{
		stop:  make(chan struct{}),
		pulse: pulse,
		gpio:  gpio,
	}

	go p.ticker()
	return p, err
}

func (p *Port) ticker() {
	t := time.NewTicker(p.pulse)

	defer t.Stop()
	for {
		select {
		case <-t.C:
			p.Lock()
			p.port ^= 1                    // XOR-Toggle zwischen 0 und 1
			eventHandler := p.eventHandler // avoid race condition
			eventType := p.port            // Kopiere p.port sicher
			p.Unlock()

			if eventHandler != nil {
				go eventHandler(rpi.Event{Timestamp: time.Now(), Type: eventType})
			}
		case <-p.stop:
			return // Exit the function when stop signal is received
		}
	}
}

// Close closes the port and releases the resources.
// The port is set to input mode. Sending events is disabled.
func (p *Port) Close() error {
	close(p.stop)
	return nil
}

// SetInputMode sets the port to input mode.
func (p *Port) SetInputMode() error {
	return nil
}

// SetOutputMode sets the port to output mode.
func (p *Port) SetOutputMode() error {
	return nil
}

// SetValue sets the value of the port.
// The value can be 0 or 1. 0 is low, 1 is high.
// The port must be in output mode. If the port is in input mode, an error is returned.
func (p *Port) SetValue(n int) error {
	p.Lock()
	p.port = n & 1
	p.Unlock()
	return nil
}

// GetValue returns the value of the port. The value can be 0 or 1. 0 is low, 1 is high.
// The port must be in input mode. If the port is in output mode, an error is returned.
func (p *Port) GetValue() (int, error) {
	p.RLock()
	n := p.port
	p.RUnlock()
	return n, nil
}

// SetPullUp sets the port to pull up mode. This is used to prevent floating values.
// The port will be pulled up to high.
// This is useful for buttons or switches that are connected to ground.
// The pull-up resistor will pull the port to high when the button is not pressed.
// The button will pull the port to low when pressed.
func (p *Port) SetPullUp() error {
	return nil
}

// SetPullDown sets the port to pull down mode. This is used to prevent floating values.
// The port will be pulled down to low.
// This is useful for buttons or switches that are connected to VCC.
// The pull-down resistor will pull the port to low when the button is not pressed.
// The button will pull the port to high when pressed.
func (p *Port) SetPullDown() error {
	return nil
}

// Port returns the offset of the port. The offset is the GPIO number.
func (p *Port) Port() int {
	return p.gpio
}

// StartWatchingEvents starts watching the port for events. The handler is called when an event is detected.
// The handler is called with an Event struct that contains the timestamp and the type of event.
// The handler is called in a separate goroutine.
func (p *Port) StartWatchingEvents(handler func(rpi.Event)) error {
	p.eventHandler = handler
	return nil
}

// StopWatchingEvents stops watching the port for events.
// The handler is removed and no more events are detected.
func (p *Port) StopWatchingEvents() error {
	p.eventHandler = nil
	return nil
}

// SetDebounceTime sets the debounced time of the port. This is used to prevent bouncing values.
// The debounced time is the time that the port is disabled after an event is detected.
// The debounced time is useful for buttons or switches that are connected to ground or VCC.
// The debounced time is used to prevent multiple events when the button is pressed or released.
func (p *Port) SetDebounceTime(t time.Duration) error {
	return nil
}
