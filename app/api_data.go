package app

import (
	"net/http"

	"github.com/womat/golib/web"
)

// HandleData returns the current meter readings and monitoring data.
//
//	@Summary		Get meter readings
//	@Description	Returns the current counter and gauge values for all registered S0 meters, keyed by meter name.
//	@Tags			s0meter
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	map[string]s0meters.MeterData	"Meter readings successfully retrieved"
//	@Failure		401	{object}	map[string]string				"Unauthorized"
//	@Router			/data [get]
func (app *App) HandleData() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			res := app.meters.GetMeterData()
			web.Encode(w, http.StatusOK, res)
		})
}
