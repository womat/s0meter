// Package app provides the main application wiring for s0meter.
//
// It initializes S0 meters, handles MQTT publishing, periodic backups,
// web server startup, and OS signal handling for graceful shutdowns
// or restarts.
//
// Usage:
//
//	config := LoadConfig()
//	app := app.New(config, "/opt/s0meter")
//	app.Run()
package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/womat/golib/mqtt"
	"github.com/womat/s0meter/app/service/s0meters"
)

// VERSION is the application version, following semantic versioning
// as described in https://semver.org/.
//
// It is not maintained in source: the Git tag is the single source of truth and
// the value is injected at build time via -ldflags (see Makefile and
// .goreleaser.yaml). The "dev" default applies to builds made without them.
var VERSION = "dev"

const (
	MODULE = "s0meter"

	ModeStop    = 0
	ModeRestart = 1
)

// App is the main application struct.
// App is where the application is wired up.
type App struct {
	wg          sync.WaitGroup // tracks the web server, backup and MQTT publish goroutines
	baseDir     string         // working directory
	config      *Config        // app configuration
	web         *http.Server   // HTTP server
	meters      *s0meters.Handler
	mqtt        *mqtt.Handler
	signals     <-chan os.Signal // OS signals, subscribed once by the caller for all lifecycles
	checkReload func() error     // loads and validates the config file before a SIGHUP restart
	restart     chan struct{}    // signals application restart
	shutdown    chan struct{}    // signals application shutdown
	ctx         context.Context
	cancelFunc  context.CancelFunc
}

// New initializes the App struct but does not start services.
//
// signals must already be subscribed (signal.Notify) to SIGHUP, SIGTERM and SIGINT, and stay
// subscribed across restarts: a signal arriving while one App is torn down and the next is
// built then waits in the channel for the next App, instead of hitting the default action,
// which would end the process without saving the counters.
//
// checkReload is called on SIGHUP before anything is torn down. If it reports an error, the
// restart is refused and the App keeps running with its current configuration, so a broken
// config file cannot stop the counting. Passing nil skips the check.
func New(config *Config, baseDir string, signals <-chan os.Signal, checkReload func() error) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		baseDir:     baseDir,
		config:      config,
		signals:     signals,
		checkReload: checkReload,
		web: &http.Server{
			Addr: net.JoinHostPort(config.Webserver.ListenHost, strconv.Itoa(config.Webserver.ListenPort)),
		},

		meters: s0meters.New(),

		restart:    make(chan struct{}),
		shutdown:   make(chan struct{}),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// Run initializes the application, starts MQTT publishing, backups,
// and the web server, and sets up OS signal handling.
func (app *App) Run() (*App, error) {
	slog.Debug("Initializing application")

	if err := app.Init(); err != nil {
		return app, err
	}

	if broker := app.config.MQTT.Connection; broker != "" {
		slog.Debug("Connecting to MQTT broker", "broker", broker)
		hostname, _ := os.Hostname()
		clientID := MODULE + hostname

		// The callbacks only log. The publish loop asks IsConnectionOpen instead, which is
		// false while a reconnect is pending; a flag set from these callbacks could go stale,
		// because the client runs them in separate goroutines.
		mqttHandler, err := mqtt.New(broker, clientID,
			mqtt.WithLogger(slog.Default()),
			mqtt.WithOnConnected(func() {
				slog.Info("MQTT connected", "broker", broker)
			}),
			mqtt.WithOnConnectionLost(func(err error) {
				slog.Warn("MQTT connection lost", "error", err)
			}))
		if err != nil {
			slog.Error("Failed to connect to MQTT broker", "broker", broker, "error", err)
			return app, err
		}

		app.mqtt = mqttHandler
		// periodically calculate the gauge- and counter-values for each meter and send the results over MQTT
		slog.Info("Starting periodic MQTT publishing",
			"heartbeat", app.config.MQTT.PublishInterval,
			"minInterval", app.config.MQTT.MinPublishInterval,
			"broker", broker)
		app.wg.Go(func() {
			app.meters.RunPeriodicPublish(app.ctx, app.config.MQTT.PublishInterval, app.config.MQTT.MinPublishInterval,
				app.mqtt, app.mqtt.IsConnectionOpen)
		})
	}

	slog.Info("Starting periodic meter data backup", "interval", app.config.BackupInterval, "file", app.config.DataFile)
	app.wg.Go(func() {
		app.meters.RunPeriodicBackup(app.ctx, app.config.BackupInterval, app.config.DataFile)
	})

	// handle the OS signals
	app.HandleOSSignals()

	slog.Info("Starting web server", "url", app.web.Addr)
	err := app.StartWebServer()
	if err != nil {
		slog.Error("Web server failed to start", "url", app.web.Addr, "error", err)
		return app, err
	}

	slog.Info("Module started successfully",
		"module", MODULE,
		"version", VERSION,
		"pid", os.Getpid(),
	)
	return app, nil
}

// Init prepares the application:
// - loads the saved pulse counts
// - adds meters, each counting on from its saved value
// - writes the data file once, which also proves it is writable
// - initializes API routes
func (app *App) Init() (err error) {

	// Read the saved values before any GPIO pin is watched, so every meter starts counting from
	// its restored value and no pulse counted during start-up can be overwritten afterwards.
	dataFile := app.config.DataFile
	slog.Info("Loading meter data", "file", dataFile)
	saved, err := s0meters.ReadMeterData(dataFile)
	if err != nil {
		slog.Error("Failed to load meter data", "file", dataFile, "error", err)
		return err
	}

	// register the meters and the GPIO pins
	for name, config := range app.config.Meter {
		slog.Info("Register meter", "name", name, "gpio", config.Gpio, "pulses", saved[name])
		if err = app.meters.RegisterMeter(app.ctx, name, config, saved[name]); err != nil {
			slog.Error("Failed to register meter", "name", name, "error", err)
			return err
		}
	}

	for name, pulses := range saved {
		if _, ok := app.config.Meter[name]; !ok {
			slog.Warn("Saved meter is not configured, its counter is dropped with the next save",
				"meter", name, "pulses", pulses, "file", dataFile)
		}
	}

	if err = app.meters.SaveMeterData(dataFile); err != nil {
		slog.Error("Failed to save meter data", "file", dataFile, "error", err)
		return err
	}

	// initRoutes should always be called at the end
	slog.Debug("Initializing API routes")
	app.SetupRoutes()

	return nil
}

// Restart returns a read-only channel for restart signals.
func (app *App) Restart() <-chan struct{} {
	return app.restart
}

// Shutdown returns a read-only channel for shutdown signals.
func (app *App) Shutdown() <-chan struct{} {
	return app.shutdown
}

// HandleOSSignals handles SIGHUP (restart), SIGTERM and SIGINT (stop) from app.signals.
//
// The subscription itself belongs to the caller and outlives this App, so nothing here
// stops or resets it; one goroutine per App consumes at most one signal.
func (app *App) HandleOSSignals() {

	go func() {
		slog.Debug("Starting signal handler")

		// Use select instead of a plain channel receive so the goroutine has
		// two exit paths and always terminates cleanly:
		//   - a signal is received and handled, or
		//   - the context is cancelled externally (e.g. from a concurrent shutdown).
		// Without the second path the goroutine would outlive its App and take
		// the next signal away from the App that replaced it. The loop only
		// continues after a SIGHUP whose config was rejected.
		for {
			select {
			case receivedSignal := <-app.signals:
				slog.Info("Received OS signal", "signal", receivedSignal)
				switch receivedSignal {
				case syscall.SIGHUP:
					if app.checkReload != nil {
						if err := app.checkReload(); err != nil {
							slog.Error("Config reload rejected, keeping the running configuration", "error", err)
							continue
						}
					}
					slog.Info("SIGHUP received, initiating restart")
					app.shutdownProcedure(ModeRestart)
				case syscall.SIGTERM, syscall.SIGINT:
					slog.Info("SIGTERM/SIGINT received, stopping")
					app.shutdownProcedure(ModeStop)
				}
				return
			case <-app.ctx.Done():
				// Context was cancelled externally – exit without triggering
				// a second shutdown procedure.
				slog.Debug("Signal handler: context cancelled, exiting goroutine")
				return
			}
		}
	}()
}

// shutdownProcedure gracefully stops or restarts the app based on mode.
//   - ModeStop: graceful shutdown the web server, Cleanup app resources and exit the application.
//   - ModeRestart: graceful shutdown the web server and Cleanup app resources and restart the application.
func (app *App) shutdownProcedure(mode int) {
	slog.Info("Initiating shutdown", "mode", mode)

	// cancel the application context to stop all running goroutines
	app.cancelFunc()
	// Wait for the web server, the backup loop and the MQTT loop, so the final save in Cleanup
	// neither races a periodic backup nor runs while a publish is still in flight.
	app.wg.Wait()

	if err := app.Cleanup(); err != nil {
		slog.Error("Cleanup failed", "error", err)
	}

	switch mode {
	case ModeRestart:
		slog.Info("Shutdown complete, restarting")
		app.restart <- struct{}{}
		// Channels are intentionally left open: cmd/main.go receives the restart
		// signal and calls New(), which creates fresh channels for the next lifecycle.
	case ModeStop:
		slog.Info("Module stopped", "module", MODULE, "version", VERSION, "pid", os.Getpid())
		app.shutdown <- struct{}{}
		close(app.shutdown)
	}

}

// Cleanup releases application resources.
// It's called when the application is shutdown or restarted.
// Should be used to free up resources.
func (app *App) Cleanup() error {
	var errs error

	if app.meters != nil {
		// Close first, so no pulse can be counted after the values have been saved.
		// The counters stay readable after Close.
		slog.Info("Closing all meters")
		errs = errors.Join(errs, app.meters.Close())

		slog.Info("Saving meter data", "file", app.config.DataFile)
		errs = errors.Join(errs, app.meters.SaveMeterData(app.config.DataFile))
	}

	if app.mqtt != nil {
		slog.Info("Disconnecting from MQTT broker")
		app.mqtt.Disconnect()
	}

	return errs
}
