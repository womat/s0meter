package app

import (
	"log/slog"
	"net/http"
	"s0counter/pkg/web"
)

// HandleData
//
//	@Summary		Get monitoring data.
//	@Description	Get monitoring data for WATCHIT.
//	@Tags			info
//	@Success		200	{object}	meters.Data
//	@Failure		400	{object}	ApiError
//	@Failure		403	{object}	ApiError
//	@Failure		422	{object}	ApiError
//	@Router			/data [get]
//	@Security		APIKeyAuth
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
