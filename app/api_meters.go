package app

import (
	"net/http"

	"github.com/womat/golib/web"
)

// HandleMeterAll returns the current meter readings and monitoring data.
//
//	@Summary		Get all meter readings
//	@Description	Returns the current counter and gauge values for all registered S0 meters, keyed by meter name.
//	@Tags			meters
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	map[string]s0meters.MeterData	"Meter readings successfully retrieved"
//	@Failure		401	{string}	string							"Unauthorized"
//	@Router			/meters [get]
func (app *App) HandleMeterAll() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			res := app.meters.GetMeterAll()
			web.Encode(w, http.StatusOK, res)
		})
}

// HandleMeterOne returns the meter reading for a specific meter.
//
//	@Summary		Get single meter reading
//	@Description	Returns the current counter and gauge values for a specific S0 meter by name.
//	@Tags			meters
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			name	path		string				true	"Meter name"
//	@Success		200		{object}	s0meters.MeterData	"Meter reading successfully retrieved"
//	@Failure		401		{string}	string				"Unauthorized"
//	@Failure		404		{string}	string				"Meter not found"
//	@Router			/meters/{name} [get]
func (app *App) HandleMeterOne() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")

			res, err := app.meters.GetMeter(name)
			if err != nil {
				web.Encode(w, http.StatusNotFound, err.Error())
				return
			}
			web.Encode(w, http.StatusOK, res)
		})
}
