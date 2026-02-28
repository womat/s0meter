// Package app sets up HTTP routes and middleware for the s0meter application.
// It supports authentication, Swagger documentation (dev only), and monitoring endpoints.
// Routes:
// - Public routes without authentication (e.g., version)
// - Protected routes requiring API key or JWT
// - Swagger documentation (only in development) at /swagger/
// - Health, Live, Ready, Monitoring, and S0 data endpoints
//
// Middleware applied:
// - CORS
// - IP filtering (allowed/blocked IPs)
//
// This must be called during app startup before starting the HTTP server.
package app

import (
	"net/http"

	"github.com/womat/golib/web"
)

// API route constants
const (
	PathVersion = "/version"
	PathHealth  = "/health"
	PathLive    = "/alive"
	PathReady   = "/ready"
	PathData    = "/data"
	PathSwagger = "/swagger/"
)

// SetupRoutes configures all HTTP routes and global middleware for the application.
func (app *App) SetupRoutes() {
	webCfg := web.Config{
		ApiKey:    app.config.HttpsServer.ApiKey,
		JwtSecret: app.config.HttpsServer.JwtSecret,
		JwtID:     app.config.HttpsServer.JwtID,
		AppName:   MODULE,
	}

	mux := http.NewServeMux()

	// Preflight CORS requests
	mux.Handle("OPTIONS /", web.HandlePreflight())

	// Dev-only Swagger documentation (only registered with -tags swagger)
	app.registerSwaggerRoute(mux)

	// Public routes
	mux.Handle("GET "+PathVersion, app.HandleVersion())
	mux.Handle("GET "+PathHealth, app.HandleHealth())

	// Protected routes
	mux.Handle("GET "+PathLive, web.WithAuth(app.HandleLive(), webCfg))
	mux.Handle("GET "+PathReady, web.WithAuth(app.HandleReady(), webCfg))
	mux.Handle("GET "+PathData, web.WithAuth(app.HandleData(), webCfg))

	// Apply global middleware: CORS + IP filter
	handler := web.WithCORS(mux)
	handler = web.WithIPFilter(handler, app.config.HttpsServer.AllowedIPs, app.config.HttpsServer.BlockedIPs)
	app.web.Handler = handler
}
