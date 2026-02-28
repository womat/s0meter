package app

import (
	"log/slog"
	"net/http"

	"github.com/womat/golib/web"
)

// HandleData returns the current meter readings and monitoring data.
//
// This endpoint requires authentication (API key) and provides
// the S0 meter data collected by the application.
//
//	@Summary		Get monitoring data
//	@Description	Retrieves meter readings for the application.
//	@Tags			monitoring
//	@Success		200	{object}	s0meters.MeterData	"Monitoring data successfully retrieved"
//	@Router			/api/data [get]
//	@Security		APIKeyAuth "API key must be provided in the header"
func (app *App) HandleData() http.Handler {

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("Incoming web request for data",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)

			res := app.meters.GetMeterData()
			web.Encode(w, http.StatusOK, res)
		})
}
