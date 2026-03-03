// This package is designed to support Kubernetes-style health checks by exposing
// separate Liveness (/live) and Readiness (/ready) endpoints. Liveness checks
// verify that the application is running, whereas Readiness checks ensure that
// the application and its dependencies (DB, GPIO, etc.) are ready to serve traffic.
//
// The health data includes metrics such as memory usage, goroutine count, application
// version, hostname, Go runtime version, and operating system.

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

// HandleReady provides a readiness check endpoint for Kubernetes.
//
// Readiness is used by Kubernetes to determine if the pod is ready to serve traffic.
// It should check dependencies such as GPIO initialization, DB connections, or other services.
//
//	@Summary		Readiness check
//	@Description	Checks if the application and its dependencies are ready to serve traffic.
//	@Tags			info
//	@Success		200	{object}	health.Model	"Application is ready"
//	@Router			/ready [get]
func (app *App) HandleReady() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if !app.meters.IsReady() {
			web.Encode(w, http.StatusServiceUnavailable, map[string]string{"error": "meters not initialized"})
			return
		}

		resp := health.GetCurrentHealth(MODULE, VERSION)
		web.Encode(w, http.StatusOK, resp)
	})
}
