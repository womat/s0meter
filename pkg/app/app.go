package app

import (
	"log/slog"
	"net/url"

	"github.com/gofiber/fiber/v2"

	"s0counter/pkg/app/config"
	"s0counter/pkg/meter"
	"s0counter/pkg/mqtt"
	"s0counter/pkg/rpi"
)

// App is the main application struct.
// App is where the application is wired up.
type App struct {
	// web is the fiber web framework instance
	web *fiber.App

	// config is the application configuration
	config *config.Config

	// urlParsed contains the parsed Config.Url parameter
	// and makes it easier to get params out of e.g.
	// url: https://0.0.0.0:7844/?minTls=1.2&bodyLimit=50MB
	urlParsed *url.URL

	// mqtt is the handler to the mqtt broker
	mqtt *mqtt.Handler

	// MetersMap must be a pointer to the Meter type, otherwise RWMutex doesn't work!
	meters map[string]*meter.Meter

	// restart signals application restart
	restart chan struct{}
	// shutdown signals application shutdown
	shutdown chan struct{}
}

// New checks the Web server URL and initializes the main app structure
func New(config *config.Config) (*App, error) {
	u, err := url.Parse(config.Webserver.URL)
	if err != nil {
		slog.Error("Error parsing url", "url", config.Webserver.URL, "error", err.Error())
		return nil, err
	}

	return &App{
		config:    config,
		urlParsed: u,

		web:    fiber.New(),
		meters: make(map[string]*meter.Meter),
		mqtt:   mqtt.New(),

		restart:  make(chan struct{}),
		shutdown: make(chan struct{}),
	}, err
}

// Run starts the application.
func (app *App) Run() error {
	if err := app.init(); err != nil {
		return err
	}

	if app.config.MQTT.Enabled {
		// periodically calculate the gauge- and counter-values for each meter and send the results over MQTT
		go app.calcGauge()
	}

	// periodically backup the measurements
	go app.backupMeasurements()

	go app.runWebServer()

	return nil
}

// init initializes the application.
func (app *App) init() (err error) {

	for meterName, meterConfig := range app.config.Meter {
		app.meters[meterName] = meter.New(meterConfig)
	}

	if err = app.loadMeasurements(); err != nil {
		slog.Error("can't open data file", "error", err)
		return err
	}

	for name, meterConfig := range app.config.Meter {
		if m, ok := app.meters[name]; ok {
			if m.LineHandler, err = rpi.NewPort(meterConfig.Gpio); err != nil {
				slog.Error("can't open port", "error", err)
				return
			}

			_ = m.LineHandler.SetInputMode()
			_ = m.LineHandler.SetDebounceTime(meterConfig.BounceTime)
			// call handler when pin changes from low to high.
			if err = m.LineHandler.StartEventWatching(m.EventHandler); err != nil {
				slog.Error("can't open watcher", "error", err)
				return err
			}
		}
	}

	if app.config.MQTT.Enabled {
		if err = app.mqtt.Connect(app.config.MQTT.Connection); err != nil {
			slog.Error("can't open mqtt broker", "error", err)
			return err
		}
	}

	// initRoutes and initDefaultRoutes should always be called last because it may access things like app.api
	// which must be initialized before in initAPI()
	app.initDefaultRoutes()

	return nil
}

// Restart returns the read only restart channel.
// Restart is used to be able to react on application restart. (see cmd/main.go)
func (app *App) Restart() <-chan struct{} {
	return app.restart
}

// Shutdown returns the read only shutdown channel.
// Shutdown is used to be able to react to application shutdown (see cmd/main.go)
func (app *App) Shutdown() <-chan struct{} {
	return app.shutdown
}

func (app *App) Close() error {

	for _, m := range app.meters {
		// close the LineHandler channel to stop the meter
		_ = m.LineHandler.Close()
	}

	if app.config.MQTT.Enabled {
		_ = app.mqtt.Disconnect()
	}

	_ = app.saveMeasurements()
	return nil
}
