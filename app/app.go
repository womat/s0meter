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
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"s0counter/app/service/s0meters"
	"s0counter/pkg/mqtt"
	"syscall"
	"time"
)

// VERSION holds the version information with the following logic in mind
//
//	4 ... fixed
//	0 ... year 2020, 1->year 2021, etc.
//	7 ... month of year (7=July)
//	the date format after the + is always the first of the month
//
// VERSION differs from semantic versioning as described in https://semver.org/
// but we keep the correct syntax.
// TODO: increase version number
const (
	VERSION = "4.5.2+20250223"
	MODULE  = "s0meter"

	ModeStop    = 0
	ModeRestart = 1
)

// App is the main application struct.
// App is where the application is wired up.
type App struct {
	baseDir    string       // working directory
	config     *Config      // app configuration
	web        *http.Server // HTTP server
	meters     *s0meters.Handler
	mqtt       *mqtt.Handler
	restart    chan struct{} // signals application restart
	shutdown   chan struct{} // signals application shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// New initializes the App struct but does not start services.
func New(config *Config, baseDir string) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		baseDir: baseDir,
		config:  config,
		web: &http.Server{
			Addr: net.JoinHostPort(config.HttpsServer.ListenHost, config.HttpsServer.ListenPort),
		},

		meters: s0meters.New(s0meters.Config{
			DataFile:               config.DataFile,
			BackupInterval:         config.BackupInterval,
			DataCollectionInterval: config.DataCollectionInterval,
		}),

		restart:    make(chan struct{}),
		shutdown:   make(chan struct{}),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// Run initializes the application, starts MQTT publishing, backups,
// and the web server, and sets up OS signal handling.
func (app *App) Run() (*App, error) {
	slog.Info("Initializing application")

	if err := app.Init(); err != nil {
		return app, err
	}

	if app.config.MQTT.Enabled {
		slog.Info("Connecting to MQTT broker", "broker", app.config.MQTT.Connection)
		hostname, _ := os.Hostname()
		clientID := MODULE + hostname

		mqttHandler, err := mqtt.New(app.config.MQTT.Connection, clientID)
		if err != nil {
			slog.Error("Failed to connect to MQTT broker", "broker", app.config.MQTT.Connection, "error", err)
			return app, err
		}

		app.mqtt = mqttHandler
		// periodically calculate the gauge- and counter-values for each meter and send the results over MQTT
		slog.Info("Starting MQTT publishing", "broker", app.config.MQTT.Connection)
		app.meters.StartPeriodicPublish(app.ctx, app.mqtt)
	}

	slog.Info("Starting periodic meter data backup", "file", app.config.DataFile)
	app.meters.StartPeriodicBackup(app.ctx)

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
// - adds meters
// - loads meter data from backup
// - initializes API routes
func (app *App) Init() (err error) {

	// register the meters and the GPIO pins
	for name, config := range app.config.Meter {
		slog.Debug("Register meter", "name", name)
		if err = app.meters.RegisterMeter(app.ctx, name, config); err != nil {
			slog.Error("Failed to register meter", "name", name, "error", err)
			return err
		}
	}

	slog.Info("Loading meter data", "file", app.config.DataFile)
	if err = app.meters.LoadMeterData(); err != nil {
		slog.Error("Failed to load meter data", "file", app.config.DataFile, "error", err)
		return err
	}

	// initRoutes should always be called at the end
	slog.Info("Initializing API routes")
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

// HandleOSSignals listens for SIGHUP, SIGTERM, and SIGINT signals.
func (app *App) HandleOSSignals() {

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

		slog.Info("Starting signal handler")

		receivedSignal := <-sig
		slog.Info("Received OS signal", "signal", receivedSignal)

		switch receivedSignal {
		case syscall.SIGHUP:
			slog.Info("SIGHUP received, initiating restart")
			app.shutdownProcedure(ModeRestart)
			// reset the signal registration before the program restarts.
			// with program restarts, the HandleOSSignals function is called again and re-registers the signals.
			signal.Reset()

		case syscall.SIGTERM, syscall.SIGINT:
			slog.Info("SIGTERM/SIGINT received, stopping")
			app.shutdownProcedure(ModeStop)
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

	if mode == ModeStop {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := app.web.Shutdown(ctx); err != nil {
			slog.Error("Web server shutdown failed", "error", err)
		}
	}

	if err := app.Cleanup(); err != nil {
		slog.Error("Cleanup failed", "error", err)
	}

	switch mode {
	case ModeRestart:
		slog.Info("Shutdown complete, restarting")
		app.restart <- struct{}{}
	case ModeStop:
		slog.Info("Module stopped", "module", MODULE, "version", VERSION, "pid", os.Getpid())
		app.shutdown <- struct{}{}
		close(app.shutdown)
		close(app.restart)
	}

}

// Cleanup releases application resources.
// It's called when the application is shutdown or restarted.
// Should be used to free up resources.
func (app *App) Cleanup() error {
	var err error

	// Close all meters and mqtt broker connection if active
	if app.meters != nil {
		slog.Info("Close all meters")
		err = app.meters.Close()
	}

	if app.mqtt != nil {
		slog.Info("Disconnecting from MQTT broker")
		app.mqtt.Disconnect()
	}

	return err

}
