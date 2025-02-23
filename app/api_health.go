package app

import (
	"github.com/womat/golib/web"
	"log/slog"
	"net/http"
	"s0counter/app/service/health"
)

// HandleHealth returns data about the health of the application.
//
//	@Summary		Get health data
//	@Description	Retrieves the health data for the application, including memory usage, goroutine count, and version.
//	@Tags			info
//	@Success		200	{object}	health.Model	"Health data successfully retrieved"
//	@Failure		403	{object}	web.ApiError	"Forbidden: Insufficient permissions"
//	@Router			/api/health [get]
func (app *App) HandleHealth() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Incoming web request for health check",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)

			resp := health.Health(VERSION)
			web.Encode(w, http.StatusOK, resp)
		},
	)
}
