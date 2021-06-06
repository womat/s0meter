package main

// https://github.com/stianeikeland/go-rpio
// https://github.com/davecheney/gpio/blob/a6de66e7e470/examples/watch/watch.go
// TODO: Use package gpiod https://github.com/warthog618/gpiod

import (
	"encoding/json"
	"gopkg.in/yaml.v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/womat/debug"
	"github.com/womat/tools"

	"s0counter/global"
	_ "s0counter/pkg/config"
	"s0counter/pkg/mqtt"
	"s0counter/pkg/raspberry"
	_ "s0counter/pkg/webservice"
)

type SavedRecord struct {
	MeterReading float64   `yaml:"meterreading"` // current meter reading (aktueller Zählerstand), eg kWh, l, m³
	TimeStamp    time.Time `yaml:"timestamp"`    // time of last s0 pulse
}
type SaveMeters map[string]SavedRecord

func main() {
	debug.SetDebug(global.Config.Debug.File, global.Config.Debug.Flag)

	for meterName, meterConfig := range global.Config.Meter {
		global.AllMeters[meterName] = &global.Meter{Config: meterConfig, UnitMeterReading: meterConfig.Unit, UnitFlow: meterConfig.UnitFlow}
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

	mqttServer := mqtt.NewMqtt()
	mqttServer.Connect(global.Config.MQTT.Connection)

	go mqttServer.Service()
	go calcFlow(global.AllMeters, global.Config.DataCollectionInterval, mqttServer.C)
	go backupMeasurements(global.Config.DataFile, global.AllMeters, global.Config.BackupInterval)

	// capture exit signals to ensure resources are released on exit.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// wait for am os.Interrupt signal (CTRL C)
	sig := <-quit
	debug.InfoLog.Printf("Got %s signal. Aborting...\n", sig)

	_ = mqttServer.Disconnect()
	for _, meter := range global.AllMeters {
		meter.LineHandler.Unwatch()
	}
	chip.Close()
	_ = saveMeasurements(global.Config.DataFile, global.AllMeters)
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
					Topic:   m.Config.MqttTopic,
					Payload: b,
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

		if err = os.WriteFile(fileName, data, 0600); err != nil {
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

	if err := os.WriteFile(fileName, data, 0600); err != nil {
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
