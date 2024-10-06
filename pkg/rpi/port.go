package rpi

import (
	"fmt"
	"log/slog"
	"time"

	gpiod "github.com/warthog618/go-gpiocdev"
)

// Chip represents a single GPIO chip that controls a set of lines.
const Chip = "gpiochip0"

const (
	// RisingEdge indicates an inactive event to an active event (low to high).
	RisingEdge = iota
	// FallingEdge indicates an active to inactive event (high to low).
	FallingEdge
)

// Port represents a single requested line.
type Port struct {
	gpioLine     *gpiod.Line
	eventHandler func(Event)
}

// Event represents a state change event on a line.
type Event struct {
	// Timestamp is the exact time when the event was detected.
	Timestamp time.Time
	// Type represents the type of state change event (RisingEdge or FallingEdge).
	Type int
}

// NewPort requests control of a single line on a chip.
func NewPort(gpio int) (*Port, error) {

	var err error
	p := &Port{}

	p.gpioLine, err = gpiod.RequestLine(
		Chip,
		gpio,
		gpiod.WithEventHandler(p.handler),
		gpiod.WithoutEdges, gpiod.AsInput)

	return p, err
}

// handler sends the event to the eventHandler.
// The eventHandler is called when an event is detected.
// The eventHandler is called with an Event struct that contains the timestamp and the type of event.
// The eventHandler is called in a separate goroutine.
// The eventHandler is set by calling StartEventWatching.
// The eventHandler is removed by calling StopEventWatching.
func (p *Port) handler(evt gpiod.LineEvent) {
	if p.eventHandler == nil ||
		(evt.Type != gpiod.LineEventFallingEdge && evt.Type != gpiod.LineEventRisingEdge) {
		return
	}

	slog.Debug("event detected",
		"timestamp", evt.Timestamp,
		"type", evt.Type,
		"offset", evt.Offset,
		"lineSeqno", evt.LineSeqno,
		"seqno", evt.Seqno,
		"port offset", p.gpioLine.Offset(),
		"port name", p.gpioLine.Chip())

	eventType := FallingEdge
	if evt.Type == gpiod.LineEventRisingEdge {
		eventType = RisingEdge
	}

	go p.eventHandler(Event{
		Timestamp: time.Now(),
		Type:      eventType,
	})
}

// Close closes the port and releases the resources.
// The port is set to input mode. Sending events is disabled.
func (p *Port) Close() error {
	_ = p.gpioLine.Reconfigure(gpiod.WithoutEdges, gpiod.AsInput)
	return p.gpioLine.Close()
}

// SetInputMode sets the port to input mode.
func (p *Port) SetInputMode() error {
	return p.gpioLine.Reconfigure(gpiod.WithoutEdges, gpiod.AsInput)
}

// SetOutputMode sets the port to output mode.
func (p *Port) SetOutputMode() error {
	return p.gpioLine.Reconfigure(gpiod.WithoutEdges, gpiod.AsOutput())
}

// SetValue sets the value of the port.
// The value can be 0 or 1. 0 is low, 1 is high.
// The port must be in output mode. If the port is in input mode, an error is returned.
func (p *Port) SetValue(n int) error {
	return p.gpioLine.SetValue(n)
}

// GetValue returns the value of the port. The value can be 0 or 1. 0 is low, 1 is high.
// The port must be in input mode. If the port is in output mode, an error is returned.
func (p *Port) GetValue() (int, error) {
	return p.gpioLine.Value()
}

// SetPullUp sets the port to pull up mode. This is used to prevent floating values.
// The port will be pulled up to high.
// This is useful for buttons or switches that are connected to ground.
// The pull-up resistor will pull the port to high when the button is not pressed.
// The button will pull the port to low when pressed.
func (p *Port) SetPullUp() error {
	return p.gpioLine.Reconfigure(gpiod.WithPullUp)
}

// SetPullDown sets the port to pull down mode. This is used to prevent floating values.
// The port will be pulled down to low.
// This is useful for buttons or switches that are connected to VCC.
// The pull-down resistor will pull the port to low when the button is not pressed.
// The button will pull the port to high when pressed.
func (p *Port) SetPullDown() error {
	return p.gpioLine.Reconfigure(gpiod.WithPullDown)
}

// Info returns the information of the port. This is useful for debugging.
// The information includes the chip name, the offset, the direction, the pull mode, and the value.
// The information is returned as a string.
func (p *Port) Info() {
	_ = fmt.Sprint(p.gpioLine.Info())
}

// StartEventWatching starts watching the port for events. The handler is called when an event is detected.
// The handler is called with an Event struct that contains the timestamp and the type of event.
// The handler is called in a separate goroutine.
func (p *Port) StartEventWatching(handler func(Event)) error {
	p.eventHandler = handler
	return p.gpioLine.Reconfigure(gpiod.WithBothEdges)
}

// StopEventWatching stops watching the port for events.
// The handler is removed and no more events are detected.
func (p *Port) StopEventWatching() error {
	p.eventHandler = nil
	return p.gpioLine.Reconfigure(gpiod.WithoutEdges)
}

// SetDebounceTime sets the debounced time of the port. This is used to prevent bouncing values.
// The debounced time is the time that the port is disabled after an event is detected.
// The debounced time is useful for buttons or switches that are connected to ground or VCC.
// The debounced time is used to prevent multiple events when the button is pressed or released.
func (p *Port) SetDebounceTime(t time.Duration) error {
	return p.gpioLine.Reconfigure(gpiod.WithDebounce(t))
}
