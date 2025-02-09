package app

import (
	"log/slog"
	"net/http"
	"s0counter/app/service/health"
	"s0counter/pkg/web"
)

// HandleHealth returns data about the health of myself.
// output example:
//
//	{"NumGoroutines":11,"HeapAllocatedBytes":332256360,"HeapAllocatedMB":316,
//	 "SysMemoryBytes":360290312,"SysMemoryMB":343,"Version":"0.0.0+20200516","ProgLang":"go1.15.2"}
//
//	@Summary		Get health
//	@Description	Get	app health data.
//	@Tags			info
//	@Success		200	{object}	health.Model
//	@Router			/health [get]
func (app *App) HandleHealth() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Incoming web request for version",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)

			resp := health.Health(VERSION)
			web.Encode(w, http.StatusOK, resp)
		},
	)
}
