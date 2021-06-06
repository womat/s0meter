package app

import (
	"time"

	"github.com/womat/debug"
)

func (app *App) increaseImpulse(pin int) {
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
