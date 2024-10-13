package main

// TODO: documentation

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"s0counter/pkg/app"
	"s0counter/pkg/app/config"
)

const defaultConfigFile = "/opt/womat/config/" + app.MODULE + ".yaml"

func main() {
	exitCode := 1
	defer func() {
		os.Exit(exitCode)
	}()

	cfg := config.NewConfig()
	debug := false

	flag.BoolVar(&cfg.Flag.Version, "version", false, "print version and exit")
	flag.BoolVar(&debug, "Debug", false, "enable debug information")
	flag.StringVar(&cfg.Flag.ConfigFile, "config", defaultConfigFile, "config file")
	flag.Parse()

	if cfg.Flag.Version {
		fmt.Println(app.Version())
		exitCode = 0
		return
	}

	if err := cfg.LoadConfig(); err != nil {
		fmt.Println(err)
		exitCode = 1
		return
	}

	if debug {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)
	}

	slog.Info(fmt.Sprintf("starting app %s", app.Version()))
	a, err := app.New(cfg)

	if err != nil {
		slog.Error("error starting	 app", "error", err.Error())
		exitCode = 1
		return
	}

	defer func() {
		slog.Info(fmt.Sprintf("closing app %s", app.Version()))
		_ = a.Close()
	}()

	err = a.Run()
	if err != nil {
		slog.Error("error running app", "error", err.Error())
		exitCode = 1
		return
	}

	// capture exit signals to ensure resources are released on exit.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// wait for am os.Interrupt signal (CTRL C)
	sig := <-quit
	slog.Info("Got %s signal. Aborting...", sig)

	exitCode = 1
	return
}
