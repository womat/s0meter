package main

// https://github.com/stianeikeland/go-rpio
// https://github.com/davecheney/gpio/blob/a6de66e7e470/examples/watch/watch.go
// TODO: Use package gpiod https://github.com/warthog618/gpiod

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"s0counter/pkg/app"
	"s0counter/pkg/app/config"
	"syscall"

	"github.com/womat/debug"
)

const defaultConfigFile = "/opt/womat/config/" + app.MODULE + ".yaml"

func main() {
	debug.SetDebug(os.Stderr, debug.Standard)
	cfg := config.NewConfig()

	flag.BoolVar(&cfg.Flag.Version, "version", false, "print version and exit")
	flag.StringVar(&cfg.Flag.Debug, "debug", "", "enable debug information (standard | trace | debug)")
	flag.StringVar(&cfg.Flag.ConfigFile, "config", defaultConfigFile, "config file")
	flag.Parse()

	if cfg.Flag.Version {
		fmt.Println(app.Version())
		os.Exit(1)
	}

	if err := cfg.LoadConfig(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	debug.SetDebug(cfg.Debug.File, cfg.Debug.Flag)

	a := app.New(cfg)
	debug.InfoLog.Printf("starting app %s", app.Version())

	if err := a.Run(); err != nil {
		debug.FatalLog.Print(err)
		os.Exit(1)
	}

	// capture exit signals to ensure resources are released on exit.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// wait for am os.Interrupt signal (CTRL C)
	sig := <-quit
	debug.InfoLog.Printf("Got %s signal. Aborting...", sig)

	a.Close()
	cfg.Debug.File.Close()

	os.Exit(1)
}
