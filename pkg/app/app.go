package app

import (
	"log"
	"net/url"
	"os"
	"s0counter/pkg/app/config"
	"s0counter/pkg/meter"
	"s0counter/pkg/mqtt"
	"s0counter/pkg/raspberry"

	"github.com/gofiber/fiber/v2"
	"github.com/womat/debug"
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

	mqtt *mqtt.Handler
	// MetersMap must be a pointer to the Meter type, otherwise RWMutex doesn't work!
	meters map[string]*meter.Meter

	chip *raspberry.Chip

	// restart signals application restart
	restart chan struct{}
	// shutdown signals application shutdown
	shutdown chan struct{}
	// power signals to check power consumption for forced activation
	power chan struct{}
	// ls signals to check consumption for load shedding
	ls chan struct{}
	// temperature signals to check temperature thresholds
	temperature chan struct{}
}

// New checks the Web server URL and initialize the main app structure
func New(config *config.Config) *App {
	u, err := url.Parse(config.Webserver.URL)
	if err != nil {
		log.Printf("Error parsing url %q: %s", config.Webserver.URL, err.Error())
		os.Exit(1)
	}

	return &App{
		config:    config,
		urlParsed: u,

		web:    fiber.New(),
		meters: meter.New(),
		mqtt:   mqtt.New(),

		restart:     make(chan struct{}),
		shutdown:    make(chan struct{}),
		power:       make(chan struct{}),
		ls:          make(chan struct{}),
		temperature: make(chan struct{}),
	}
}

// Run starts the application.
func (app *App) Run() error {
	if err := app.init(); err != nil {
		return err
	}

	for _, m := range app.meters {
		go testPinEmu(m.LineHandler)
	}

	go app.mqtt.Service()
	go app.calcFlow()
	go app.backupMeasurements()
	go app.runWebServer()

	select {}
	// return nil
}

// init initializes the application.
func (app *App) init() (err error) {
	for meterName, meterConfig := range app.config.Meter {
		app.meters[meterName] = &meter.Meter{
			Config:           meterConfig,
			UnitMeterReading: meterConfig.Unit,
			UnitFlow:         meterConfig.UnitFlow,
		}
	}

	if err = app.loadMeasurements(); err != nil {
		debug.ErrorLog.Printf("can't open data file: %v", err)
		// os.Exit(1)
		return err
	}

	if app.chip, err = raspberry.Open(); err != nil {
		debug.ErrorLog.Printf("can't open gpio: %v\n", err)
		// os.Exit(1)
		return err
	}

	for name, meterConfig := range app.config.Meter {
		if m, ok := app.meters[name]; ok {
			if m.LineHandler, err = app.chip.NewPin(meterConfig.Gpio); err != nil {
				debug.ErrorLog.Printf("can't open pin: %v", err)
				return
			}

			app.meters[name] = m
			app.meters[name].LineHandler.Input()
			app.meters[name].LineHandler.PullUp()
			app.meters[name].LineHandler.SetBounceTime(meterConfig.BounceTime)
			// call handler when pin changes from low to high.
			if err = app.meters[name].LineHandler.Watch(raspberry.EdgeFalling, app.handler); err != nil {
				debug.ErrorLog.Printf("can't open watcher: %v", err)
				return err
			}
		}
	}

	if err = app.mqtt.Connect(app.config.MQTT.Connection); err != nil {
		debug.ErrorLog.Printf("can't open mqtt broker %v", err)
		return err
	}

	// initRoutes and initDefaultRoutes should be always called last because it may access things like app.api
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
// Shutdown is used to be able to react on application shutdown. (see cmd/main.go)
func (app *App) Shutdown() <-chan struct{} {
	return app.shutdown
}

func (app *App) Close() error {
	for _, m := range app.meters {
		m.LineHandler.Unwatch()
	}

	app.mqtt.Disconnect()
	app.chip.Close()
	return app.saveMeasurements()
}
