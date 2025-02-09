package app

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"s0counter/app/service/s0meters"
	"syscall"
	"time"
)

// VERSION holds the version information with the following logic in mind
//
//	1 ... fixed
//	0 ... year 2020, 1->year 2021, etc.
//	7 ... month of year (7=July)
//	the date format after the + is always the first of the month
//
// VERSION differs from semantic versioning as described in https://semver.org/
// but we keep the correct syntax.
// TODO: increase version number to 1.0.1+2020xxyy
const (
	VERSION = "3.3.5+2021013"
	MODULE  = "s0counter"
)

//go:embed README.md
var Readme string

// App is the main application struct.
// App is where the application is wired up.
type App struct {

	// baseDir is the application working directory
	baseDir string

	// config is the application configuration
	config *Config

	// web is the web server.
	web *http.Server

	// meters is the handler to the meters
	meters *s0meters.Handler

	// restart signals application restart
	restart chan struct{}

	// shutdown signals application shutdown
	shutdown chan struct{}
}

// New checks the Web server URL and initializes the main app structure
func New(config *Config, baseDir string) *App {

	return &App{
		baseDir: baseDir,
		config:  config,
		web:     &http.Server{},

		meters: s0meters.New(s0meters.Config{
			DataFile:               config.DataFile,
			BackupInterval:         config.BackupInterval,
			DataCollectionInterval: config.DataCollectionInterval,
			MqttRetained:           config.MQTT.Retained,
		}),

		restart:  make(chan struct{}),
		shutdown: make(chan struct{}),
	}
}

// Run starts the application.
func (app *App) Run() error {
	slog.Info("Initializing application")

	if err := app.Init(); err != nil {
		return err
	}

	if app.config.MQTT.Enabled {
		// periodically calculate the gauge- and counter-values for each meter and send the results over MQTT
		slog.Info("Starting MQTT publishing", "broker", app.config.MQTT.Connection)
		go app.meters.PublishMetrics()
	}

	// periodically backup the measurements
	slog.Info("Starting periodic meter data backup", "file", app.config.DataFile)
	go app.meters.BackupMeterData()

	app.runSignalHandler()

	webServerAddress := net.JoinHostPort(app.config.Webserver.ListenHost, app.config.Webserver.ListenPort)
	slog.Info("Starting web server", "url", webServerAddress)
	err := app.runWebServer()
	if err != nil {
		slog.Error("Web server failed to start", "url", webServerAddress, "error", err)

		return err
	}

	slog.Info(fmt.Sprintf("%s started successfully", MODULE), "version", VERSION, "pid", os.Getpid())
	return nil
}

// Init initializes the application.
func (app *App) Init() (err error) {

	// add the meters
	for name, config := range app.config.Meter {
		slog.Debug("Adding meter", "name", name)
		if err = app.meters.AddMeter(name, config); err != nil {
			slog.Error("Failed to add meter", "name", name, "error", err)
			return err
		}
	}

	// load the meter data from the YAML file
	slog.Info("Loading meter data", "file", app.config.DataFile)
	if err = app.meters.LoadMeterData(); err != nil {
		slog.Error("Failed to load meter data", "file", app.config.DataFile, "error", err)
		return err
	}

	// connect to the mqtt broker
	if app.config.MQTT.Enabled {
		slog.Info("Connecting to MQTT broker", "broker", app.config.MQTT.Connection)
		if err = app.meters.Connect(app.config.MQTT.Connection); err != nil {
			slog.Error("Failed to connect to MQTT broker", "broker", app.config.MQTT.Connection, "error", err)
			return err
		}
	}

	// initRoutes should always be called at the end
	slog.Info("Initializing API routes")
	app.InitRoutes()

	return nil
}

// Restart returns the read-only restart channel.
// Restart is used to be able to react on application restart. (see cmd/main.go)
func (app *App) Restart() <-chan struct{} {
	return app.restart
}

// Shutdown returns the read-only shutdown channel.
// Shutdown is used to be able to react to application shutdown (see cmd/main.go)
func (app *App) Shutdown() <-chan struct{} {
	return app.shutdown
}

// runSignalHandler runs the os signal handler to react on os signals (SIGHUP, SIGTERM, SIGINT).
func (app *App) runSignalHandler() {

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

		slog.Info("Starting signal handler")

		receivedSignal := <-sig
		slog.Warn("Received OS signal", "signal", receivedSignal)

		switch receivedSignal {
		case syscall.SIGHUP:
			slog.Info("SIGHUP received, initiating restart")
			app.shutdownProcedure("restart")
			signal.Reset()

		case syscall.SIGTERM:
			slog.Info("SIGTERM received, gracefully shutting down")
			app.shutdownProcedure("shutdown")

		case syscall.SIGINT:
			slog.Info("SIGINT received, exiting")
			app.shutdownProcedure("terminate")
		}

	}()
}

// Handles both SIGTERM and SIGINT (graceful shutdown)
func (app *App) shutdownProcedure(mode string) {
	slog.Info("Initiating shutdown", "mode", mode)

	if mode == "shutdown" || mode == "restart" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := app.web.Shutdown(ctx); err != nil {
			slog.Error("Web server shutdown failed", "error", err)
		}
	}

	if err := app.cleanup(); err != nil {
		slog.Error("Cleanup failed", "error", err)
	}

	if mode == "restart" {
		slog.Info("Shutdown completed, preparing to restart")
		app.restart <- struct{}{}
		return
	}

	slog.Info(fmt.Sprintf("%s stopped", MODULE), "version", VERSION, "pid", os.Getpid())
	app.shutdown <- struct{}{}
	close(app.shutdown)
	close(app.restart)
}

// cleanup free's application resources.
// It's called when application is shutdown or restarted.
// Should be used to free up resources.
func (app *App) cleanup() error {

	//err := app.meters.Close()
	return nil
}
