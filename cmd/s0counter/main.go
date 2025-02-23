package main

import (
	_ "embed"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"s0counter/app"
	"s0counter/pkg/xlog"
)

//go:embed README.md
var Readme string

func main() {
	// Parse command line flags.
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	about := flags.Bool("about", false, "Print app details and exit")
	help := flags.Bool("help", false, "Print a help message and exit")
	version := flags.Bool("version", false, "Print the app version and exit")

	logLevel := flags.String("logLevel", "", "Set the log level (overrides the config file). Supported values: debug | info | warning | error")
	logDestination := flags.String("logDestination", "", "Set the log destination (overrides the config file). Supported values: stdout | stderr | null | /path/to/logfile")
	configFile := flags.String("config", filepath.Join("/opt", app.MODULE, "etc/config.yaml"), "Specify the path to the config file")

	if err := flags.Parse(os.Args[1:]); err != nil {
		fmt.Printf("error: %s\n", err.Error())
		os.Exit(1)
	}

	switch {
	case *about:
		fmt.Println(About())
		os.Exit(0)
	case *version:
		fmt.Println(app.VERSION)
		os.Exit(0)
	case *help:
		fmt.Println(Readme)
		os.Exit(0)
	}

	var logger *xlog.LoggerWrapper

	config, err := loadConfig(*configFile, *logLevel, *logDestination)
	if err != nil {
		fmt.Printf("Failed to load config file %s: %s\n", *configFile, err.Error())
		os.Exit(1)
	}

	for {
		// run the app in a function to be able to restart it and reload the config
		// possible open log files are always closed before the function exits
		func() {
			if logger, err = xlog.Init(config.LogDestination, config.LogLevel); err != nil {
				fmt.Printf("Failed to initialize logger: %s\n", err.Error())
				os.Exit(1)
			}
			defer logger.Close()

			// set slog logger as default logger
			slog.SetDefault(logger.Logger)
			slog.Info("Logging initialized", "logLevel", config.LogLevel)
			slog.Debug("Starting with configuration", "config", config)

			a, err := app.New(config, filepath.Join("/opt", app.MODULE)).Run()
			if err != nil {
				slog.Error("Critical error occurred, shutting down", "error", err)
				os.Exit(1)
			}

			select {
			case <-a.Restart():
				slog.Info("Reloading configuration", "configFile", *configFile)
				if config, err = loadConfig(*configFile, *logLevel, *logDestination); err != nil {
					slog.Error("Failed to reload config file, shutting down",
						"configFile", *configFile,
						"error", err)
					os.Exit(1)
				}

			case <-a.Shutdown():
				os.Exit(0)
			}
		}()
	}
}

func About() string {
	p := map[string]string{
		"Author":   "Wolfgang Mathe",
		"Binary":   "/opt/s0counter/bin/s0counter",
		"Comment":  "config .env file see /opt/s0counter/.env  and config file /opt/s0counter/etc/config.yaml",
		"Date":     "2024-10-04",
		"Desc":     "s0counter reads impulses from an S0 interface compliant with DIN 43864 standards",
		"Help":     "/opt/s0counter/bin/s0counter -help",
		"Libinfo":  "plain go with go modules from ITdesign golib",
		"Main":     "/opt/src/s0counter/cmd/app/main.go",
		"ProgLang": runtime.Version(),
		"Repo":     " https://github.com/womat/s0counter.git",
		"Version":  app.VERSION,
	}
	b, _ := yaml.Marshal(p)
	return string(b)
}

// loadConfig loads the configuration from the given file.
func loadConfig(configFile, logLevel, logDestination string) (*app.Config, error) {

	config, err := app.NewConfig().LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	switch logLevel {
	case "": // if no log level is provided, use the one from the config
	case "debug", "info", "warning", "error":
		config.LogLevel = logLevel
	default:
		return nil, fmt.Errorf("invalid log level: %s", logLevel)
	}

	// Set log destination if provided
	if logDestination != "" {
		config.LogDestination = logDestination
	}

	return config, nil
}
