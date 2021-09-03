package app

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/womat/debug"
)

type resp struct {
	TimeStamp   time.Time // timestamp of last gauge calculation
	Counter     float64   // current counter (aktueller Zählerstand), eg kWh, l, m³
	UnitCounter string    // unit of current meter counter e.g. kWh, l, m³
	Gauge       float64   // mass flow rate per time unit  (= counter/time(h)), e.g. kW, l/h, m³/h
	UnitGauge   string    // unit of gauge, eg Wh, l/s, m³/h
}

// runWebServer starts the applications web server and listens for web requests.
//  It's designed to run in a separate go function to not block the main go function.
//  e.g.: go runWebServer()
//  See app.Run()
func (app *App) runWebServer() {
	err := app.web.Listen(app.urlParsed.Host)
	debug.ErrorLog.Print(err)
}

func (app *App) HandleCurrentData() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		debug.InfoLog.Print("web request currentdata")

		res := map[string]resp{}
		for n, m := range app.meters {
			m.RLock()
			res[n] = resp{
				TimeStamp:   time.Now(),
				Counter:     calcCounter(m),
				UnitCounter: m.Config.UnitCounter,
				Gauge:       calcGauge(m),
				UnitGauge:   m.Config.UnitGauge,
			}
			m.RUnlock()
		}

		return ctx.JSON(res)
	}
}
