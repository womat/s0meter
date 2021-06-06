package app

import (
	"s0counter/pkg/raspberry"
	"time"

	"github.com/womat/debug"
)

func testPinEmu(p raspberry.Pin) {
	if p.HW() == raspberry.Raspberrypi {
		return
	}

	for range time.Tick(time.Duration(p.Pin()/2) * time.Second) {
		raspberry.TestPin(p, raspberry.EdgeFalling)
	}
}

func (app *App) handler(p raspberry.Pin) {
	pin := p.Pin()

	for _, m := range app.meters {
		// find the measuring device based on the pin configuration
		if m.Config.Gpio == pin {
			// add current counter & set time stamp
			debug.DebugLog.Printf("receive an impulse on pin: %v", pin)

			func() {
				m.Lock()
				defer m.Unlock()
				m.MeterReading += m.Config.ScaleFactor
				m.S0.Counter++
				m.S0.TimeStamp = time.Now()
			}()

			return
		}
	}
}
