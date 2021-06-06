package raspberry

import (
	"time"
)

// Edge represents the change in Pin level that triggers an interrupt.
type Edge string

const (
	// EdgeNone indicates no level transitions will trigger an interrupt
	EdgeNone Edge = "none"

	// EdgeRising indicates an interrupt is triggered when the Pin transitions from low to high.
	EdgeRising Edge = "rising"

	// EdgeFalling indicates an interrupt is triggered when the Pin transitions from high to low.
	EdgeFalling Edge = "falling"

	// EdgeBoth indicates an interrupt is triggered when the Pin changes level.
	EdgeBoth Edge = "both"

	// supported HW
	Raspberrypi = iota
	Windows
)

type Chip interface {
	Close() error
	NewPin(int) (Pin, error)
}

type Pin interface {
	SetBounceTime(time.Duration)
	BounceTime() time.Duration
	Watch(Edge, func(Pin)) error
	Unwatch()
	Input()
	PullUp()
	PullDown()
	Pin() int
	Read() bool
	LastLevel() bool
	SetLastLevel(bool)
	BounceTimer() *time.Timer
	Edge() Edge
	Handler() func(Pin)
	HW() int
}

func Handler(pin Pin) {
	// if debounce is inactive, call handler function and returns
	if pin.BounceTime() == 0 {
		pin.SetLastLevel(pin.Read())
		handler := pin.Handler()
		handler(pin)
		return
	}

	select {
	case <-pin.BounceTimer().C:
		// if bounce Timer is expired, accept new signals
		pin.BounceTimer().Reset(pin.BounceTime())
	default:
		// if bounce Timer is still running, ignore single
		return
	}

	go func(l Pin) {
		// wait until bounce Timer is expired and check if the pin has still the correct level
		// the correct level depends on the edge configuration
		<-l.BounceTimer().C
		l.BounceTimer().Reset(0)

		switch l.Edge() {
		case EdgeBoth:
			if pin.Read() != l.LastLevel() {
				l.SetLastLevel(pin.Read())
				handler := pin.Handler()
				handler(pin)
			}
		case EdgeFalling:
			if !l.Read() {
				l.SetLastLevel(pin.Read())
				handler := pin.Handler()
				handler(pin)
			}
		case EdgeRising:
			if l.Read() {
				l.SetLastLevel(pin.Read())
				handler := pin.Handler()
				handler(pin)
			}
		}
	}(pin)
	return
}

func TestPin(p Pin, edge Edge) {
	switch {
	case p.Edge() == EdgeNone, edge == EdgeNone:
		return

	case edge == EdgeBoth:
		// if edge is EdgeBoth, handler is called twice
		if p.Edge() == EdgeBoth {
			handler := p.Handler()
			handler(p)
		}

		if p.Edge() == EdgeBoth || p.Edge() == EdgeFalling || p.Edge() == EdgeRising {
			handler := p.Handler()
			handler(p)
		}
	case edge == EdgeFalling:
		if p.Edge() == EdgeBoth || p.Edge() == EdgeFalling {
			handler := p.Handler()
			handler(p)
		}
	case edge == EdgeRising:
		if p.Edge() == EdgeBoth || p.Edge() == EdgeRising {
			handler := p.Handler()
			handler(p)
		}
	}
}
