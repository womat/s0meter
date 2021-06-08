package app

import (
	"github.com/gofiber/fiber/v2"
	"github.com/womat/debug"
)

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

		// Lock all Meters to marshal data
		for _, m := range app.meters {
			m.RLock()
			defer m.RUnlock()
		}

		return ctx.JSON(app.meters)
	}
}
