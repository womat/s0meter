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
	"sort"
	"strings"

	"github.com/womat/golib/web"
)

// API route constants
const (
	PathVersion = "/api/version"
	PathHealth  = "/api/health"
	PathLive    = "/api/live"
	PathReady   = "/api/ready"
	PathData    = "/api/data"
	PathSwagger = "/swagger/"
)

// RouteMap maps HTTP methods (GET, POST, etc.) to their handlers.
// Returns 405 if the method is not allowed.
type RouteMap struct {
	handlers map[string]http.Handler
	allowed  []string // cached list of allowed methods
}

// NewRouteMap creates a RouteMap from a map of handlers.
func NewRouteMap(h map[string]http.Handler) *RouteMap {
	methods := make([]string, 0, len(h))
	for m := range h {
		methods = append(methods, strings.ToUpper(m))
	}
	sort.Strings(methods)
	return &RouteMap{
		handlers: h,
		allowed:  methods,
	}
}

// SetupRoutes configures all HTTP routes and global middleware for the application.
func (app *App) SetupRoutes() {
	webCfg := web.Config{
		ApiKey:    app.config.HttpsServer.ApiKey,
		JwtSecret: app.config.HttpsServer.JwtSecret,
		JwtID:     app.config.HttpsServer.JwtID,
		AppName:   MODULE,
	}

	mux := http.NewServeMux()
	mux.Handle("OPTIONS /", web.HandlePreflight())

	// Dev-only Swagger documentation

	// Dev-only Swagger documentation (only registered with -tags swagger)
	app.registerSwaggerRoute(mux)

	// Public routes
	mux.Handle(PathVersion, NewRouteMap(map[string]http.Handler{"GET": app.HandleVersion()}))
	mux.Handle(PathHealth, NewRouteMap(map[string]http.Handler{"GET": app.HandleHealth()}))

	// Protected routes
	mux.Handle(PathLive, NewRouteMap(map[string]http.Handler{"GET": web.WithAuth(app.HandleLive(), webCfg)}))
	mux.Handle(PathReady, NewRouteMap(map[string]http.Handler{"GET": web.WithAuth(app.HandleReady(), webCfg)}))
	mux.Handle(PathData, NewRouteMap(map[string]http.Handler{"GET": web.WithAuth(app.HandleData(), webCfg)}))

	// Apply global middleware: CORS + IP filter
	handler := web.WithCORS(mux)
	handler = web.WithIPFilter(handler, app.config.HttpsServer.AllowedIPs, app.config.HttpsServer.BlockedIPs)
	app.web.Handler = handler
}

// ServeHTTP implements http.Handler for RouteMap.
// It automatically checks the HTTP method and returns 405 if the method is not allowed.
func (rm *RouteMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h, ok := rm.handlers[strings.ToUpper(r.Method)]; ok {
		h.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Allow", strings.Join(rm.AllowedMethods(), ", "))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// AllowedMethods returns a sorted slice of allowed HTTP methods for this route.
func (rm *RouteMap) AllowedMethods() []string {
	return rm.allowed
}
