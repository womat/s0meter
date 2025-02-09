package app

import (
	"log/slog"
	"net/http"
	"s0counter/app/service/monitoring"
	"s0counter/pkg/web"
)

// HandleMonitoring
//
//	@Summary		Get monitoring data.
//	@Description	Get monitoring data for WATCHIT.
//	@Tags			info
//	@Success		200	{object}	[]monitoring.Model
//	@Failure		400	{object}	ApiError
//	@Failure		403	{object}	ApiError
//	@Failure		422	{object}	ApiError
//	@Router			/monitoring [get]
//	@Security		APIKeyAuth
func (app *App) HandleMonitoring() http.Handler {

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Info("Incoming web request for monitoring",
				"method", r.Method,
				"path", r.URL.Path,
				"client_ip", r.RemoteAddr)

			resp, err := monitoring.Monitoring(r.Host, VERSION)
			if err != nil {
				web.Encode(w, http.StatusInternalServerError, web.NewApiError(err))
				return
			}

			web.Encode(w, http.StatusOK, resp)
		},
	)
}
