package app

import (
	"log/slog"
	"net/http"

	"github.com/womat/golib/web"
)

// HandleVersion returns the application name and version.
//
//	@Summary		Get application version and name
//	@Description	Returns the name and version of the application for debugging and monitoring.
//	@Tags			info
//	@Success		200	{object}	object{app=string,appVersion=string}	"Application version and name successfully retrieved"
//	@Router			/api/version [get]
func (app *App) HandleVersion() http.Handler {
	// Response defines the JSON structure returned by /api/version
	type HandleVersionResponse struct {
		App        string `json:"app"`        // Application name (MODULE)
		AppVersion string `json:"appVersion"` // Application version (VERSION)
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("Incoming web request for version info",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)
			web.Encode(w, http.StatusOK, HandleVersionResponse{App: MODULE, AppVersion: VERSION})
		})
}
