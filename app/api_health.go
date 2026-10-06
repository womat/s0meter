package app

// HTTP handlers for health and readiness: /health returns runtime metrics, /ready is a
// readiness probe for monitoring.

import (
	"net/http"
	"time"

	"github.com/womat/golib/web"
	"github.com/womat/s0meter/app/service/health"
)

// HandleHealth returns the current health data of the application.
//
//	@Summary		Get health data
//	@Description	Retrieves memory usage, goroutine count, version, hostname, Go runtime version, OS, the MQTT connection state, and per meter the raw pulses, the time and age of the last pulse, and the GPIO events lost (counted late, the gauge skips the gap).
//	@Tags			info
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	health.Model	"Health data successfully retrieved"
//	@Failure		401	{string}	string			"Unauthorized"
//	@Router			/health [get]
func (app *App) HandleHealth() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			resp := health.GetCurrentHealth(MODULE, VERSION)
			resp.Mqtt = app.mqttState()
			resp.Meters = app.meters.Status(time.Now())
			web.Encode(w, http.StatusOK, resp)
		},
	)
}

// mqttState reports the broker connection for /health. Like /ready it looks at
// IsConnectionOpen, which is false during a reconnect as well.
func (app *App) mqttState() string {
	switch {
	case app.mqtt == nil:
		return health.MqttDisabled
	case app.mqtt.IsConnectionOpen():
		return health.MqttConnected
	default:
		return health.MqttDisconnected
	}
}

// HandleReady is a readiness probe for monitoring.
// It returns 200 OK while the service can deliver readings, and 503 Service Unavailable
// while an MQTT broker is configured but no connection to it is open, because readings
// are then not delivered. Without a broker the service is always ready.
//
//	@Summary		Readiness check
//	@Description	Returns 200 while the service delivers readings, 503 while a configured MQTT broker is not connected. No authentication required.
//	@Tags			info
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Application is ready"
//	@Failure		503	{object}	map[string]string	"MQTT broker not connected"
//	@Router			/ready [get]
func (app *App) HandleReady() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.mqtt != nil && !app.mqtt.IsConnectionOpen() {
			web.Encode(w, http.StatusServiceUnavailable, map[string]string{"error": "mqtt broker not connected"})
			return
		}

		web.Encode(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}
