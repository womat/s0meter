package app

import (
	"log/slog"
	"net/http"
	"s0counter/pkg/web"
)

// HandleVersion returns the version of the application.
// It is used for health checks and debugging.
//
//	@Summary		Get version
//	@Description	Get	app version and name.
//	@Tags			info
//	@Success		200	{object}	app.HandleVersion.Response
//	@Router			/version [get]
func (app *App) HandleVersion() http.Handler {
	type Response struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("Incoming web request for version",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)
			web.Encode(w, http.StatusOK, Response{Version: VERSION, Name: MODULE})
		})
}
