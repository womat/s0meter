// +build !windows

package old

import (
	"s0counter/pkg/raspberry"

	"github.com/warthog618/gpio"
)

func testPinEmu(l *raspberry.Line) {
}

func handler(pin *gpio.Pin) {
	increaseImpulse(pin.Pin())
}
