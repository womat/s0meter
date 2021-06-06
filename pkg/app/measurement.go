package app

import (
	"encoding/json"
	"os"
	"s0counter/pkg/mqtt"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/womat/debug"
	"github.com/womat/tools"
)

type SavedRecord struct {
	MeterReading float64   `yaml:"meterreading"` // current meter reading (aktueller Zählerstand), eg kWh, l, m³
	TimeStamp    time.Time `yaml:"timestamp"`    // time of last s0 pulse
}
type SaveMeters map[string]SavedRecord

func (app *App) calcFlow() {
	p := app.config.DataCollectionInterval
	for range time.Tick(p) {
		debug.DebugLog.Println("calc average values")

		for _, m := range app.meters {
			func() {
				m.Lock()
				defer m.Unlock()
				m.Flow = float64(m.S0.Counter-m.S0.LastCounter) / p.Hours() * m.Config.ScaleFactor * m.Config.ScaleFactorFlow
				m.S0.LastCounter = m.S0.Counter
				m.TimeStamp = time.Now()
				b, err := json.MarshalIndent(m, "", "  ")
				if err != nil {
					debug.ErrorLog.Println(err)
					return
				}

				app.mqtt.C <- mqtt.Message{
					Qos:      0,
					Retained: true,
					Topic:    m.Config.MqttTopic,
					Payload:  b,
				}
			}()
		}
	}
}

func (app *App) loadMeasurements() (err error) {
	// if file doesn't exists, create an empty file
	fileName := app.config.DataFile
	if !tools.FileExists(fileName) {
		s := SaveMeters{}

		for name := range app.meters {
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
		if meter, ok := app.meters[name]; ok {
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

func (app *App) backupMeasurements() {
	for range time.Tick(app.config.BackupInterval) {
		_ = app.saveMeasurements()
	}
}

func (app *App) saveMeasurements() error {
	debug.DebugLog.Println("saveMeasurements measurements to file")

	s := SaveMeters{}

	for name, m := range app.meters {
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

	if err := os.WriteFile(app.config.DataFile, data, 0o600); err != nil {
		debug.ErrorLog.Printf("backupMeasurements write file: %v", err)
		return err
	}

	return nil
}
