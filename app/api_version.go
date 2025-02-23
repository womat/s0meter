package app

import (
	"github.com/womat/golib/web"
	"log/slog"
	"net/http"
)

// HandleVersion returns the version and name of the application.
//
//	@Summary		Get application version and name
//	@Description	This endpoint returns the name and version of the application to help with debugging and monitoring.
//	@Tags			info
//	@Success		200	{object}	app.HandleVersion.Response	"Application version and name successfully retrieved"
//	@Router			/api/version [get]
func (app *App) HandleVersion() http.Handler {
	type Response struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("Incoming web request for version info",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)
			web.Encode(w, http.StatusOK, Response{Version: VERSION, Name: MODULE})
		})
}
