// Package app provides HTTP handlers for application health and readiness checks.
// Health returns runtime metrics; Ready is a Kubernetes readiness probe.

package app

import (
	"net/http"
	"s0meter/app/service/health"

	"github.com/womat/golib/web"
)

// HandleHealth returns the current health data of the application.
//
//	@Summary		Get health data
//	@Description	Retrieves memory usage, goroutine count, version, hostname, Go runtime version, and OS.
//	@Tags			info
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	health.Model	"Health data successfully retrieved"
//	@Failure		401	{string}	string			"Unauthorized"
//	@Router			/health [get]
func (app *App) HandleHealth() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			resp := health.GetCurrentHealth(MODULE, VERSION)
			web.Encode(w, http.StatusOK, resp)
		},
	)
}

// HandleReady is a Kubernetes readiness probe endpoint.
// It returns 200 OK when all dependencies are initialized and ready to serve traffic,
// or 503 Service Unavailable if any dependency (e.g. meters) is not yet ready.
//
//	@Summary		Readiness check
//	@Description	Returns 200 if all dependencies are ready, 503 otherwise. No authentication required.
//	@Tags			info
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Application is ready"
//	@Failure		503	{object}	map[string]string	"Service unavailable"
//	@Router			/ready [get]
func (app *App) HandleReady() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.meters.IsReady() {
			web.Encode(w, http.StatusServiceUnavailable, map[string]string{"error": "meters not initialized"})
			return
		}

		web.Encode(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}
