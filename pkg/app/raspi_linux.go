// +build !windows

package app

import (
	"s0counter/pkg/raspberry"

	"github.com/warthog618/gpio"
)

func testPinEmu(l *raspberry.Line) {
}

func (app *App) handler(pin *gpio.Pin) {
	app.increaseImpulse(pin.Pin())
}
