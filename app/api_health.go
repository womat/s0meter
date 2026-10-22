package app

// HTTP handler for /health: runtime metrics, the MQTT connection and the state of every meter.

import (
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/womat/golib/v2/web"
	"github.com/womat/s0meter/app/service/s0meters"
)

// Health is the response of /health: system and runtime information, the MQTT connection and
// the diagnostic state of every meter.
type Health struct {
	App            string  `json:"app"`            // Application name (MODULE)
	AppVersion     string  `json:"appVersion"`     // Current version of the application
	GoVersion      string  `json:"goVersion"`      // Go runtime version
	Hostname       string  `json:"hostname"`       // Machine name where the app runs
	OS             string  `json:"os"`             // Operating system name
	UptimeSeconds  float64 `json:"uptimeSeconds"`  // Application uptime in seconds
	NumGoroutines  int     `json:"numGoroutines"`  // Current number of active goroutines
	HeapAllocBytes uint64  `json:"heapAllocBytes"` // Allocated heap memory in bytes
	SysMemoryBytes uint64  `json:"sysMemoryBytes"` // Total memory obtained from the OS
	Timestamp      string  `json:"timestamp"`      // UTC timestamp when health info was collected (RFC3339)

	// Mqtt is the state of the broker connection: connected, disconnected (also while
	// reconnecting) or disabled when no broker is configured.
	Mqtt string `json:"mqtt"`

	// MqttBroker is the configured broker as host:port, for the tooltip of the MQTT pill on
	// the web page. Never user or password from the connection URL; omitted without a broker.
	MqttBroker string `json:"mqttBroker,omitempty"`

	// Meters holds the diagnostic state per meter name: raw pulses, the last pulse and the
	// GPIO events lost since the last start or reload.
	Meters map[string]s0meters.MeterStatus `json:"meters"`
}

// States of Health.Mqtt.
const (
	mqttConnected    = "connected"
	mqttDisconnected = "disconnected"
	mqttDisabled     = "disabled"
)

var startTime = time.Now() // start of the process, for UptimeSeconds; a reload does not reset it

// HandleHealth returns the current health data of the application.
//
//	@Summary		Get health data
//	@Description	Retrieves memory usage, goroutine count, version, hostname, Go runtime version, OS, the MQTT connection state and broker (host:port, never credentials), and per meter the raw pulses, the time and age of the last pulse, the GPIO events lost (counted late, the gauge skips the gap), the MQTT topic and the reading in the display units of the web UI.
//	@Tags			info
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	Health			"Health data successfully retrieved"
//	@Failure		401	{object}	web.ApiError	"Unauthorized"
//	@Router			/health [get]
func (app *App) HandleHealth() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			host, err := os.Hostname()
			if err != nil {
				host = "unknown"
			}
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)

			resp := Health{
				App:            MODULE,
				AppVersion:     VERSION,
				GoVersion:      runtime.Version(),
				Hostname:       host,
				OS:             runtime.GOOS,
				UptimeSeconds:  time.Since(startTime).Seconds(),
				NumGoroutines:  runtime.NumGoroutine(),
				HeapAllocBytes: mem.Alloc,
				SysMemoryBytes: mem.Sys,
				Timestamp:      time.Now().UTC().Format(time.RFC3339),
				Mqtt:           app.mqttState(),
				Meters:         app.meters.Status(time.Now()),
			}
			if app.config.MQTT != nil {
				resp.MqttBroker = brokerHost(app.config.MQTT.Connection)
			}
			web.Encode(w, http.StatusOK, resp)
		},
	)
}

// mqttState reports the broker connection for /health. It looks at IsConnectionOpen, which is
// false during a reconnect as well.
func (app *App) mqttState() string {
	switch {
	case app.mqtt == nil:
		return mqttDisabled
	case app.mqtt.IsConnectionOpen():
		return mqttConnected
	default:
		return mqttDisconnected
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
