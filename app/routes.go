package app

import (
	httpSwagger "github.com/swaggo/http-swagger"
	"net/http"
	"s0counter/pkg/web"
)

// InitRoutes initializes and configures all HTTP routes for the application.
// It sets up authentication, Swagger documentation, and middleware (CORS, IP filtering).
// - Public routes without authentication
// - Protected routes with authentication
// - Swagger documentation available at /swagger/
// - Adds global middleware for CORS and IP filtering.
//
// This function must be called during application startup before the web server is launched.
func (app *App) InitRoutes() {

	webCfg := web.Config{
		ApiKey:    app.config.HttpsServer.ApiKey,
		JwtSecret: app.config.HttpsServer.JwtSecret,
		JwtID:     app.config.HttpsServer.JwtID,
		AppName:   MODULE,
	}

	mux := http.NewServeMux()
	mux.Handle("OPTIONS /", web.HandlePreflight())

	if app.config.IsDevEnv() {
		// Expose Swagger documentation only in development.
		mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.PersistAuthorization(true)))
	}

	mux.Handle("GET /api/version", app.HandleVersion())
	mux.Handle("GET /api/health", app.HandleHealth())
	mux.Handle("GET /api/monitoring", web.WithAuth(app.HandleMonitoring(), webCfg))
	mux.Handle("GET /api/data", web.WithAuth(app.HandleData(), webCfg))

	// Global middleware is added here.
	app.web.Handler = web.WithCORS(mux)
	app.web.Handler = web.WithIPFilter(app.web.Handler, app.config.HttpsServer.AllowedIPs, app.config.HttpsServer.BlockedIPs)
}
