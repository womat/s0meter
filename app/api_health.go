package app

// HTTP handler for /health: runtime metrics, the MQTT connection and the state of every meter.

import (
	"net/http"
	"net/url"
	"time"

	"github.com/womat/golib/v2/web"
	"github.com/womat/s0meter/app/service/health"
)

// HandleHealth returns the current health data of the application.
//
//	@Summary		Get health data
//	@Description	Retrieves memory usage, goroutine count, version, hostname, Go runtime version, OS, the MQTT connection state and broker (host:port, never credentials), and per meter the raw pulses, the time and age of the last pulse, the GPIO events lost (counted late, the gauge skips the gap), the MQTT topic and the reading in the display units of the web UI.
//	@Tags			info
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	health.Model	"Health data successfully retrieved"
//	@Failure		401	{object}	web.ApiError	"Unauthorized"
//	@Router			/health [get]
func (app *App) HandleHealth() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			resp := health.GetCurrentHealth(MODULE, VERSION)
			resp.Mqtt = app.mqttState()
			if app.config.MQTT != nil {
				resp.MqttBroker = brokerHost(app.config.MQTT.Connection)
			}
			resp.Meters = app.meters.Status(time.Now())
			web.Encode(w, http.StatusOK, resp)
		},
	)
}

// mqttState reports the broker connection for /health. It looks at IsConnectionOpen, which is
// false during a reconnect as well.
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

// brokerHost is the broker as the web page shows it: host and port, never a user or password
// someone wrote into the connection URL.
func brokerHost(connection string) string {
	u, err := url.Parse(connection)
	if err != nil {
		return ""
	}
	return u.Host
}
