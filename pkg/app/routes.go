package app

// initDefaultRoutes initializes the application default routes.
//
//	These are the routes that always are the same in every application.
//	Things like user api, version, ...
func (app *App) initDefaultRoutes() {
	api := app.web.Group("/")
	api.Get("/version", app.HandleVersion())
	api.Get("/health", app.HandleHealth())
	api.Get("/currentdata", app.HandleCurrentData())
}
