package main

// https://github.com/stianeikeland/go-rpio
// https://github.com/davecheney/gpio/blob/a6de66e7e470/examples/watch/watch.go
// TODO: Use package gpiod https://github.com/warthog618/gpiod

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"s0counter/global"
	"s0counter/pkg/app"
	"s0counter/pkg/app/config"
	"s0counter/pkg/mqtt"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/womat/debug"
	"github.com/womat/tools"

	_ "s0counter/pkg/config"

	_ "s0counter/pkg/webservice"
)

type SavedRecord struct {
	MeterReading float64   `yaml:"meterreading"` // current meter reading (aktueller Zählerstand), eg kWh, l, m³
	TimeStamp    time.Time `yaml:"timestamp"`    // time of last s0 pulse
}
type SaveMeters map[string]SavedRecord

const defaultConfigFile = "/opt/womat/conf/" + app.MODULE + ".yaml"

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
		fmt.Print(err)
		os.Exit(1)
	}

	debug.SetDebug(cfg.Debug.File, cfg.Debug.Flag)

	a := app.New(cfg)
	debug.InfoLog.Printf("starting app %s", app.Version())

	if err := a.Run(); err != nil {
		debug.FatalLog.Print(err)
		os.Exit(1)
	}

	/*
		for meterName, meterConfig := range global.Config.Meter {
			global.AllMeters[meterName] = &global.Meter{
				Config:           meterConfig,
				UnitMeterReading: meterConfig.Unit,
				UnitFlow:         meterConfig.UnitFlow,
			}
		}

		if err := loadMeasurements(global.Config.DataFile, global.AllMeters); err != nil {
			debug.ErrorLog.Printf("can't open data file: %v\n", err)
			os.Exit(1)
			return
		}

		chip, err := raspberry.Open()
		if err != nil {
			debug.ErrorLog.Printf("can't open gpio: %v\n", err)
			os.Exit(1)
			return
		}
		defer chip.Close()

		for name, meterConfig := range global.Config.Meter {
			if meter, ok := global.AllMeters[name]; ok {
				if meter.LineHandler, err = chip.NewPin(meterConfig.Gpio); err != nil {
					debug.ErrorLog.Printf("can't open pin: %v\n", err)
					return
				}

				global.AllMeters[name] = meter
				global.AllMeters[name].LineHandler.Input()
				global.AllMeters[name].LineHandler.PullUp()
				global.AllMeters[name].LineHandler.SetBounceTime(meterConfig.BounceTime)
				// call handler when pin changes from low to high.
				if err = global.AllMeters[name].LineHandler.Watch(raspberry.EdgeFalling, handler); err != nil {
					debug.ErrorLog.Printf("can't open watcher: %v\n", err)
					return
				}

				defer global.AllMeters[name].LineHandler.Unwatch()
				go testPinEmu(global.AllMeters[name].LineHandler)
			}
		}

		mqttServer := mqtt.New()
		mqttServer.Connect(global.Config.MQTT.Connection)

		go mqttServer.Service()
		go calcFlow(global.AllMeters, global.Config.DataCollectionInterval, mqttServer.C)
		go backupMeasurements(global.Config.DataFile, global.AllMeters, global.Config.BackupInterval)
	*/

	// capture exit signals to ensure resources are released on exit.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// wait for am os.Interrupt signal (CTRL C)
	sig := <-quit
	debug.InfoLog.Printf("Got %s signal. Aborting...\n", sig)

	//	_ = mqttServer.Disconnect()
	//	for _, meter := range global.AllMeters {
	//		meter.LineHandler.Unwatch()
	//	}

	a.Close()
	// chip.Close()

	//	_ = saveMeasurements(global.Config.DataFile, global.AllMeters)
	os.Exit(1)
}

func calcFlow(meters global.MetersMap, period time.Duration, c chan mqtt.Message) {
	for range time.Tick(period) {
		debug.DebugLog.Println("calc average values")

		for _, m := range meters {
			func() {
				m.Lock()
				defer m.Unlock()
				m.Flow = float64(m.S0.Counter-m.S0.LastCounter) / period.Hours() * m.Config.ScaleFactor * m.Config.ScaleFactorFlow
				m.S0.LastCounter = m.S0.Counter
				m.TimeStamp = time.Now()
				b, err := json.MarshalIndent(m, "", "  ")
				if err != nil {
					debug.ErrorLog.Println(err)
					return
				}

				c <- mqtt.Message{
					Qos:      0,
					Retained: true,
					Topic:    m.Config.MqttTopic,
					Payload:  b,
				}
			}()
		}
	}
}

func loadMeasurements(fileName string, allMeters global.MetersMap) (err error) {
	// if file doesn't exists, create an empty file
	if !tools.FileExists(fileName) {
		s := SaveMeters{}

		for name := range allMeters {
			s[name] = SavedRecord{MeterReading: 0, TimeStamp: time.Time{}}
		}

		// marshal the byte slice which contains the yaml file's content into SaveMeters struct
		var data []byte
		data, err = yaml.Marshal(&s)
		if err != nil {
			return
		}

		if err = os.WriteFile(fileName, data, 0o600); err != nil {
			return
		}
	}

	// read the yaml file as a byte array.
	data, err := os.ReadFile(fileName)
	if err != nil {
		return
	}

	// unmarshal the byte slice which contains the yaml file's content into SaveMeters struct
	s := SaveMeters{}
	if err = yaml.Unmarshal(data, &s); err != nil {
		return
	}

	for name, loadedMeter := range s {
		if meter, ok := allMeters[name]; ok {
			func() {
				meter.Lock()
				defer meter.Unlock()
				meter.MeterReading = loadedMeter.MeterReading
				meter.TimeStamp = loadedMeter.TimeStamp
			}()
		}
	}

	return
}

func backupMeasurements(fileName string, meters global.MetersMap, period time.Duration) {
	for range time.Tick(period) {
		_ = saveMeasurements(fileName, meters)
	}
}

func saveMeasurements(fileName string, meters global.MetersMap) error {
	debug.DebugLog.Println("saveMeasurements measurements to file")

	s := SaveMeters{}

	for name, m := range meters {
		func() {
			m.RLock()
			defer m.RUnlock()
			s[name] = SavedRecord{MeterReading: m.MeterReading, TimeStamp: m.TimeStamp}
		}()
	}

	// marshal the byte slice which contains the yaml file's content into SaveMeters struct
	data, err := yaml.Marshal(&s)
	if err != nil {
		debug.ErrorLog.Printf("backupMeasurements marshal: %v\n", err)
		return err
	}

	if err := os.WriteFile(fileName, data, 0o600); err != nil {
		debug.ErrorLog.Printf("backupMeasurements write file: %v\n", err)
		return err
	}

	return nil
}

func increaseImpulse(pin int) {
	for _, m := range global.AllMeters {
		// find the measuring device based on the pin configuration
		if m.Config.Gpio == pin {
			// add current counter & set time stamp
			debug.DebugLog.Printf("receive an impulse on pin: %v\n", pin)

			func() {
				m.Lock()
				defer m.Unlock()
				m.MeterReading += m.Config.ScaleFactor
				m.S0.Counter++
				m.S0.TimeStamp = time.Now()
			}()

			return
		}
	}
}
