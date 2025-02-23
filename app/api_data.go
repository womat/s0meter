package app

import (
	"github.com/womat/golib/web"
	"log/slog"
	"net/http"
)

// HandleData returns monitoring data for the application.
//
//	@Summary		Get monitoring data
//	@Description	Retrieves the monitoring data for the WATCHIT application, including meter readings.
//	@Tags			info
//	@Success		200	{object}	s0meters.Data	"Monitoring data successfully retrieved"
//	@Failure		403	{object}	web.ApiError	"Forbidden: Insufficient permissions"
//	@Router			/api/data [get]
//	@Security		APIKeyAuth	"API key must be provided in the header"
func (app *App) HandleData() http.Handler {

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Incoming web request for data",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)

			res := app.meters.GetMeters()
			web.Encode(w, http.StatusOK, res)
		})
}
