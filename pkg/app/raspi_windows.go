// +build windows

package app

import (
	"s0counter/pkg/raspberry"
	"time"
)

func testPinEmu(l *raspberry.Line) {
	for range time.Tick(time.Duration(l.Pin()/2) * time.Second) {
		l.TestPin(raspberry.EdgeFalling)
	}
}

func (app *App) handler(l *raspberry.Line) {
	app.increaseImpulse(l.Pin())
}
