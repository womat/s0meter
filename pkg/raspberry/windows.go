//+build windows

package raspberry

import (
	"fmt"
	"time"
)

type WinPin struct {
	gpioPin int
	edge    Edge
	// the bounceTime defines the key bounce time (ms)
	// the value 0 ignores key bouncing
	bounceTime time.Duration
	// while bounceTimer is running, new signal are ignored (suppress key bouncing)
	bounceTimer *time.Timer
	lastLevel   bool
	handler     func(Pin)
}

type WinChip struct {
	pins map[int]Pin
}

func Open() (*WinChip, error) {
	return &WinChip{pins: map[int]Pin{}}, nil
}

func (c *WinChip) Close() error {
	return nil
}

func (c *WinChip) NewPin(p int) (Pin, error) {
	if _, ok := c.pins[p]; ok {
		return nil, fmt.Errorf("pin %v already used", p)
	}

	l := WinPin{gpioPin: p, bounceTimer: time.NewTimer(0)}
	c.pins[p] = &l
	return c.pins[p], nil
}

func (p *WinPin) Watch(edge Edge, handler func(Pin)) error {
	p.handler = handler
	p.edge = edge
	return nil
}

func (p *WinPin) Unwatch() {
}

func (p *WinPin) SetBounceTime(t time.Duration) {
	p.bounceTime = t
	return
}

func (p *WinPin) BounceTimer() *time.Timer {
	return p.bounceTimer
}

func (p *WinPin) BounceTime() time.Duration {
	return p.bounceTime
}

func (p *WinPin) SetLastLevel(b bool) {
	p.lastLevel = b
}

func (p *WinPin) LastLevel() bool {
	return p.lastLevel
}

func (p *WinPin) Edge() Edge {
	return p.edge
}

func (p *WinPin) Handler() func(Pin) {
	return p.handler
}

func (p *WinPin) Input() {
}

func (p *WinPin) PullUp() {
}

func (p *WinPin) PullDown() {
}

func (p *WinPin) Pin() int {
	return p.gpioPin
}

func (p *WinPin) Read() bool {
	return false
}

func (p *WinPin) HW() int {
	return Windows
}
