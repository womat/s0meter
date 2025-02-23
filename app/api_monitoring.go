package app

import (
	"github.com/womat/golib/web"
	"log/slog"
	"net/http"
	"s0counter/app/service/monitoring"
)

// HandleMonitoring returns monitoring data for WATCHIT system.
//
//	@Summary		Get monitoring data for WATCHIT
//	@Description	This endpoint returns monitoring data for the WATCHIT system, including health, performance metrics, and system status.
//	@Tags			info
//	@Success		200	{object}	[]monitoring.Model	"Monitoring data successfully retrieved"
//	@Failure		403	{object}	web.ApiError		"Forbidden: Insufficient permissions"
//	@Failure		500	{object}	web.ApiError		"Internal server error"
//	@Router			/api/monitoring [get]
//	@Security		APIKeyAuth 		"API key must be provided in the header"
func (app *App) HandleMonitoring() http.Handler {

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Incoming web request for monitoring info",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)

			resp, err := monitoring.Monitoring(r.Host, VERSION)
			if err != nil {
				slog.Error("Error retrieving monitoring data", "error", err)
				web.Encode(w, http.StatusInternalServerError, web.NewApiError(err))
				return
			}

			web.Encode(w, http.StatusOK, resp)
		},
	)
}
