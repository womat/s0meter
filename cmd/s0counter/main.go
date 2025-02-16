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
	"s0counter/pkg/crypt"
	"s0counter/pkg/xlog"
)

//go:embed README.md
var Readme string

func main() {
	// Parse command line flags.
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(os.Stdout)

	about := flags.Bool("about", false, "print app about details and exit")
	cryptString := flags.String("crypt", "", "encrypt the given string and exit")
	help := flags.Bool("help", false, "print a help message and exit")
	version := flags.Bool("version", false, "print app version and exit")

	trace := flags.Bool("trace", false, "same as --debug but with added source code location in log messages")
	debug := flags.Bool("debug", false, "enable debug information")
	configFile := flags.String("config", app.DefaultConfigFile, "config file")

	if err := flags.Parse(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}

	// Print about details and exit when about flag is used.
	if *about {
		printAbout()
		os.Exit(0)
	}

	// Encrypt the given string and exit when crypt flag is used.
	if *cryptString != "" {
		fmt.Println(crypt.NewEncryptedString(*cryptString).EncryptedValue())
		os.Exit(0)
	}

	if *version {
		fmt.Println(app.VERSION)
		os.Exit(0)
	}

	if *help {
		fmt.Println(Readme)
		os.Exit(0)
	}

	var logger *xlog.LoggerWrapper
	var config *app.Config
	var err error

	for {
		// run the app in a function to be able to restart it and reload the config
		// possible open log files are always closed before the function exits
		func() {
			if config, err = loadConfig(*configFile, *debug, *trace); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to load config file %s: %s\n", *configFile, err.Error())
				os.Exit(1)
			}

			if logger, err = xlog.Init(config.LogDestination, config.LogLevel); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %s\n", err.Error())
				os.Exit(1)
			}
			defer logger.Close()

			// set slog logger as default logger
			slog.SetDefault(logger.Logger)
			slog.Info("Logging initialized", "logLevel", config.LogLevel)
			slog.Debug("Starting with configuration", "config", config)

			a := app.New(config, filepath.Join("/opt", app.MODULE))
			err = a.Run()
			if err != nil {
				slog.Error("Critical error occurred, shutting down", "error", err)
				os.Exit(1)
			}

			select {
			case <-a.Restart():
				slog.Info("Reloading configuration", "configFile", *configFile)
				if config, err = loadConfig(*configFile, *debug, *trace); err != nil {
					slog.Error("Failed to reload configuration, shutting down",
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

func printAbout() {
	type ProgInfo struct {
		Author   string `yaml:"author"`
		Binary   string `yaml:"binary"`
		Comment  string `yaml:"comment"`
		Date     string `yaml:"date"`
		Desc     string `yaml:"desc"`
		Help     string `yaml:"help"`
		Libinfo  string `yaml:"libinfo"`
		Main     string `yaml:"main"`
		ProgLang string `yaml:"progLang"`
		Repo     string `yaml:"repo"`
		Version  string `yaml:"version"`
	}

	var p = ProgInfo{
		Author:   "Wolfgang Mathe",
		Binary:   "/opt/s0counter/bin/s0counter",
		Comment:  "config .env file see /opt/s0counter/.env  and config file /opt/s0counter/etc/config.yaml",
		Date:     "2024-10-04",
		Desc:     "s0counter reads impulses from an S0 interface compliant with DIN 43864 standards",
		Help:     "/opt/s0counter/bin/s0counter --help",
		Libinfo:  "plain go with go modules from ITdesign golib",
		Main:     "/opt/src/s0counter/cmd/app/main.go",
		ProgLang: runtime.Version(),
		Repo:     " https://github.com/womat/s0counter.git",
		Version:  app.VERSION,
	}
	b, _ := yaml.Marshal(p)
	fmt.Print(string(b))
}

// loadConfig loads the configuration from the given file.
func loadConfig(configFile string, debug, trace bool) (*app.Config, error) {

	config, err := app.NewConfig().LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	// add stdout to log destinations if debug or trace is set
	if debug || trace {
		config.LogDestination = "stdout"
		config.LogLevel = "debug"

		if trace {
			config.LogLevel = "trace"
		}
	}

	return config, nil
}
