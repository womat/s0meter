// Package app provides the main application wiring for s0meter.
//
// It initializes the S0 meters, MQTT publishing, periodic backups, the HTTPS API and the
// OS signal handling for graceful stops and configuration reloads. One App lives for one
// configuration; cmd/main.go builds a new one on every reload.
//
// Usage:
//
//	cfg, err := app.LoadConfig(file)          // then cfg.Validate()
//	signals := make(chan os.Signal, 1)
//	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
//	a, err := app.New(cfg, signals, checkReload).Run()
//	<-a.Done()
//	if a.Restarting() { /* build the next App */ } else { /* exit */ }
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/womat/golib/v2/web"
	"github.com/womat/mqtt"
	"github.com/womat/s0meter/app/service/s0meters"
)

// VERSION is the application version, following semantic versioning
// as described in https://semver.org/.
//
// It is not maintained in source: the Git tag is the single source of truth and
// the value is injected at build time via -ldflags (see Makefile and
// .goreleaser.yaml). The "dev" default applies to builds made without them.
var VERSION = "dev"

const MODULE = "s0meter"

// mode is what shutdownProcedure does after the App has stopped.
type mode string

const (
	modeStop    mode = "stop"    // the process exits
	modeRestart mode = "restart" // cmd builds the next App with the reloaded configuration
)

// ErrPermanent marks a start failure that a restart cannot fix, such as a damaged data file or
// a TLS certificate that cannot be loaded: it needs someone to repair it. cmd ends s0meter with exit code 78 for it, so systemd leaves
// the service stopped instead of restarting it into the same error.
var ErrPermanent = errors.New("restarting does not help")

// App is where the application is wired up: one App lives for one configuration.
type App struct {
	wg sync.WaitGroup // tracks the web server, backup and MQTT publish goroutines
	// metersLoaded is set once setup registered every meter with its saved counter; only then
	// may cleanup write the data file.
	metersLoaded bool
	config       *Config      // app configuration
	web          *http.Server // HTTP server
	meters       *s0meters.Handler
	mqtt         *mqtt.Handler
	signals      <-chan os.Signal // OS signals, subscribed once by the caller for all lifecycles
	checkReload  func() error     // loads and validates the config file before a SIGHUP restart
	serverErr    chan error       // reports a web server that stopped on its own
	done         chan struct{}    // closed once the App has stopped, see Done
	restarting   bool             // set before done is closed, see Restarting
	ctx          context.Context
	cancelFunc   context.CancelFunc
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
func New(config *Config, signals <-chan os.Signal, checkReload func() error) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		config:      config,
		signals:     signals,
		checkReload: checkReload,
		web: &http.Server{
			Addr: net.JoinHostPort(config.Webserver.ListenHost, strconv.Itoa(config.Webserver.ListenPort)),
		},

		meters: s0meters.New(),

		serverErr:  make(chan error, 1),
		done:       make(chan struct{}),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// Run initializes the application, starts MQTT publishing, backups,
// and the web server, and sets up OS signal handling.
func (app *App) Run() (*App, error) {
	slog.Debug("Initializing application")

	if err := app.setup(); err != nil {
		return app, app.abort(err)
	}

	if mqttConfig := app.config.MQTT; mqttConfig != nil {
		broker := mqttConfig.Connection
		// The broker URL may carry credentials (tcp://user:password@host); never log them.
		logBroker := redactURL(broker)
		slog.Debug("Connecting to MQTT broker", "broker", logBroker)
		hostname, err := os.Hostname()
		if err != nil {
			slog.Warn("Cannot read the host name, the MQTT client ID is not unique", "error", err)
		}
		clientID := MODULE + hostname

		// The callbacks only log. The publish loop asks IsConnectionOpen instead, which is
		// false while a reconnect is pending; a flag set from these callbacks could go stale,
		// because the client runs them in separate goroutines.
		mqttHandler, err := mqtt.New(broker, clientID,
			mqtt.WithLogger(slog.Default()),
			mqtt.WithOnConnected(func() {
				slog.Info("MQTT connected", "broker", logBroker)
			}),
			mqtt.WithOnConnectionLost(func(err error) {
				slog.Warn("MQTT connection lost", "error", err)
			}))
		// womat/mqtt v1.0.1 returns no error here: a broker that cannot be reached is retried
		// in the background, and the publish loop waits for IsConnectionOpen. The branch stays
		// for a later version whose New can fail.
		if err != nil {
			slog.Error("Failed to connect to MQTT broker", "broker", logBroker, "error", err)
			return app, app.abort(err)
		}

		app.mqtt = mqttHandler
		// periodically calculate the gauge- and counter-values for each meter and send the results over MQTT
		slog.Info("Starting periodic MQTT publishing",
			"heartbeat", mqttConfig.PublishInterval,
			"minInterval", mqttConfig.MinInterval(),
			"broker", logBroker)
		app.wg.Go(func() {
			app.meters.RunPeriodicPublish(app.ctx, mqttConfig.PublishInterval, mqttConfig.MinInterval(), app.mqtt)
		})
	}

	slog.Info("Starting periodic meter data backup", "interval", app.config.BackupInterval, "file", app.config.DataFile)
	app.wg.Go(func() {
		app.meters.RunPeriodicBackup(app.ctx, app.config.BackupInterval, app.config.DataFile)
	})

	slog.Info("Starting web server", "url", app.web.Addr)
	err := app.startWebServer()
	if err != nil {
		slog.Error("Web server failed to start", "url", app.web.Addr, "error", err)
		return app, app.abort(err)
	}

	// Handle the OS signals only once nothing can fail any more. A signal that is already
	// waiting would otherwise start a shutdown alongside abort, and be lost to the next App.
	app.handleSignals()

	slog.Info("Module started successfully",
		"module", MODULE,
		"version", VERSION,
		"pid", os.Getpid(),
	)
	return app, nil
}

// setup prepares the application:
// - loads the saved pulse counts
// - adds meters, each counting on from its saved value
// - writes the data file once, which also proves it is writable
// - initializes API routes
func (app *App) setup() (err error) {

	// Read the saved values before any GPIO pin is watched, so every meter starts counting from
	// its restored value and no pulse counted during start-up can be overwritten afterwards.
	dataFile := app.config.DataFile
	slog.Info("Loading meter data", "file", dataFile)
	saved, err := s0meters.ReadMeterData(dataFile)
	if err != nil {
		slog.Error("Failed to load meter data", "file", dataFile, "error", err)
		return fmt.Errorf("%w (%w)", err, ErrPermanent)
	}

	// register the meters and the GPIO pins
	for name, config := range app.config.Meter {
		slog.Info("Register meter", "name", name, "gpio", config.Gpio, "pulses", saved[name])
		if err = app.meters.RegisterMeter(name, config, saved[name]); err != nil {
			slog.Error("Failed to register meter", "name", name, "error", err)
			return err
		}
	}

	// A meter that was commented out keeps its counter, so enabling it again continues from it.
	for name, pulses := range saved {
		if _, ok := app.config.Meter[name]; !ok {
			slog.Warn("Saved meter is not configured, its counter is kept in the data file",
				"meter", name, "pulses", pulses, "file", dataFile)
			app.meters.KeepSaved(name, pulses)
		}
	}

	// From here on the meters hold every saved counter, so cleanup may save them. Saving after a
	// failed registration would drop the counters of the meters not registered yet.
	app.metersLoaded = true

	if err = app.meters.SaveMeterData(dataFile); err != nil {
		slog.Error("Failed to save meter data", "file", dataFile, "error", err)
		return err
	}

	// Checked on every start and reload, so a fixed layout stops the warning without a restart.
	// A warning, not an error: a development checkout always looks like that.
	if err = web.CheckTokenFile(app.config.Webserver.TokenFile); err != nil {
		slog.Warn("Token file is not protected", "error", err)
	}

	slog.Debug("Initializing API routes")
	app.setupRoutes()
	app.logTokens()

	return nil
}

// logTokens names the bearer tokens the protected routes accept, or warns that there are none.
func (app *App) logTokens() {
	names := make([]string, 0, len(app.config.Webserver.Tokens))
	for _, t := range app.config.Webserver.Tokens {
		names = append(names, t.Name)
	}
	if len(names) == 0 {
		slog.Warn("No bearer tokens loaded, every request to /health and /meters is rejected",
			"tokenFile", app.config.Webserver.TokenFile)
		return
	}
	slog.Info("Bearer tokens loaded", "tokenFile", app.config.Webserver.TokenFile, "names", names)
}

// Done is closed once the App has stopped and released everything: after SIGHUP, SIGTERM or
// SIGINT, or a web server that stopped on its own. Restarting then tells what comes next.
func (app *App) Done() <-chan struct{} {
	return app.done
}

// Restarting reports, once Done is closed, whether the App stopped to be replaced by one with
// the reloaded configuration (SIGHUP, web server error); false means the process exits.
func (app *App) Restarting() bool {
	return app.restarting
}

// handleSignals handles SIGHUP (restart), SIGTERM and SIGINT (stop) from app.signals, and
// restarts the App when the web server stopped on its own.
//
// The subscription itself belongs to the caller and outlives this App, so nothing here
// stops or resets it; one goroutine per App consumes at most one signal. Being the only
// caller of shutdownProcedure, it also rules out two shutdowns running at once.
func (app *App) handleSignals() {

	go func() {
		slog.Debug("Starting signal handler")

		// Use select instead of a plain channel receive so the goroutine has
		// two exit paths and always terminates cleanly:
		//   - a signal or a server error is received and handled, or
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
					app.shutdownProcedure(modeRestart)
				case syscall.SIGTERM, syscall.SIGINT:
					slog.Info("SIGTERM/SIGINT received, stopping")
					app.shutdownProcedure(modeStop)
				}
				return
			case err := <-app.serverErr:
				slog.Error("Web server stopped unexpectedly, initiating restart", "error", err)
				app.shutdownProcedure(modeRestart)
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

// shutdownProcedure stops the App and closes Done; m decides what Restarting reports. Closing
// instead of sending means it never blocks on a caller that is not listening.
func (app *App) shutdownProcedure(m mode) {
	slog.Info("Initiating shutdown", "mode", m)
	app.stop()

	app.restarting = m == modeRestart
	if app.restarting {
		slog.Info("Shutdown complete, restarting")
	} else {
		slog.Info("Module stopped", "module", MODULE, "version", VERSION, "pid", os.Getpid())
	}
	close(app.done)
}

// cleanup releases application resources.
// It's called when the application is shutdown or restarted.
// Should be used to free up resources.
func (app *App) cleanup() error {
	// Close first, so no pulse can be counted after the values have been saved. The counters
	// stay readable after Close.
	slog.Info("Closing all meters")
	err := app.meters.Close()

	// After a half-way setup the meters lack some saved counters: keep the data file as it is.
	if app.metersLoaded {
		slog.Info("Saving meter data", "file", app.config.DataFile)
		err = errors.Join(err, app.meters.SaveMeterData(app.config.DataFile))
	}

	if app.mqtt != nil {
		slog.Info("Disconnecting from MQTT broker")
		app.mqtt.Disconnect()
	}

	return err
}

// abort undoes a Run that failed half way: it stops the goroutines already started and
// releases what setup acquired, so the caller can start another App with the same GPIO lines,
// port and data file. It returns err.
func (app *App) abort(err error) error {
	app.stop()
	return err
}

// stop cancels the App's context, waits for the web server, the backup loop and the MQTT loop,
// and then releases the resources, so the final save in cleanup neither races a periodic backup
// nor runs while a publish is still in flight.
func (app *App) stop() {
	app.cancelFunc()
	app.wg.Wait()
	if err := app.cleanup(); err != nil {
		slog.Error("Cleanup failed", "error", err)
	}
}

// redactURL returns broker with a password replaced by "xxxxx", for logging.
func redactURL(broker string) string {
	u, err := url.Parse(broker)
	if err != nil {
		return "<unparsable broker URL>"
	}
	return u.Redacted()
}
