package app

import (
	httpSwagger "github.com/swaggo/http-swagger"
	"net/http"
	"s0counter/pkg/web"
)

// addRoutes is called by the main function to set up the routes.
// The mux is the main router for the application.
func (app *App) InitRoutes() {

	webCfg := web.Config{
		ApiKey:    app.config.ApiKey,
		JwtSecret: app.config.JwtSecret,
		JwtID:     app.config.JwtID,
		AppName:   MODULE,
	}

	mux := http.NewServeMux()
	mux.Handle("OPTIONS /", web.HandlePreflight())
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.PersistAuthorization(true)))

	mux.Handle("GET /api/version", app.HandleVersion())
	mux.Handle("GET /api/health", app.HandleHealth())
	mux.Handle("GET /api/monitoring", web.WithAuth(app.HandleMonitoring(), webCfg))
	mux.Handle("GET /api/data", web.WithAuth(app.HandleData(), webCfg))

	// Global middleware is added here.
	app.web.Handler = web.WithCORS(mux)
	app.web.Handler = web.WithIPFilter(app.web.Handler, app.config.Webserver.AllowedIPs, app.config.Webserver.BlockedIPs)
}
