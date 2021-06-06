// +build !windows

package raspberry

import (
	"fmt"
	"time"

	"github.com/warthog618/gpio"
)

type RpiPin struct {
	gpioPin *gpio.Pin
	edge    Edge
	// the bounceTime defines the key bounce time (ms)
	// the value 0 ignores key bouncing
	bounceTime time.Duration
	// while bounceTimer is running, new signal are ignored (suppress key bouncing)
	bounceTimer *time.Timer
	lastLevel   bool
	handler     func(Pin)
}

// must be global in package, because handler function handler(pin *gpio.Pin) need this line Infos
var pins map[int]Pin

type RpiChip struct{}

func Open() (*RpiChip, error) {
	pins = map[int]Pin{}

	if err := gpio.Open(); err != nil {
		return nil, err
	}
	return &RpiChip{}, nil
}

func (c *RpiChip) Close() (err error) {
	return gpio.Close()
}

func (c *RpiChip) NewPin(p int) (Pin, error) {
	if _, ok := pins[p]; ok {
		return nil, fmt.Errorf("pin %v already used", p)
	}

	l := RpiPin{gpioPin: gpio.NewPin(p), bounceTimer: time.NewTimer(0)}
	pins[p] = &l
	return pins[p], nil
}

func (p *RpiPin) Watch(edge Edge, handler func(Pin)) error {
	p.handler = handler
	p.edge = edge
	p.gpioPin.Mode()
	return p.gpioPin.Watch(gpio.Edge(edge), rpiHandler)
}

func (p *RpiPin) Unwatch() {
	p.gpioPin.Unwatch()
}

func (p *RpiPin) BounceTime() time.Duration {
	return p.bounceTime
}

func (p *RpiPin) SetBounceTime(t time.Duration) {
	p.bounceTime = t
	return
}

func (p *RpiPin) BounceTimer() *time.Timer {
	return p.bounceTimer
}

func (p *RpiPin) LastLevel() bool {
	return p.lastLevel
}

func (p *RpiPin) SetLastLevel(b bool) {
	p.lastLevel = b == true
}

func (p *RpiPin) Input() {
	p.gpioPin.Input()
}

func (p *RpiPin) PullUp() {
	p.gpioPin.PullUp()
}

func (p *RpiPin) PullDown() {
	p.gpioPin.PullDown()
}

func (p *RpiPin) Pin() int {
	return p.gpioPin.Pin()
}

func (p *RpiPin) Read() bool {
	return bool(p.gpioPin.Read())
}

func (p *RpiPin) Handler() func(Pin) {
	return p.handler
}

func (p *RpiPin) Edge() Edge {
	return p.edge
}

func rpiHandler(pin *gpio.Pin) {
	// check if map with pin struct exists
	p, ok := pins[pin.Pin()]
	if !ok {
		return
	}

	Handler(p)
}

func (p *RpiPin) HW() int {
	return Raspberrypi
}
