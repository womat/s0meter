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
	Counter   float64   `yaml:"counter"`   // current meter counter (aktueller Zählerstand), eg kWh, l, m³
	TimeStamp time.Time `yaml:"timestamp"` // time of last s0 pulse
}
type SaveMeters map[string]SavedRecord

type MQTTRecord struct {
	TimeStamp   time.Time // timestamp of last gauge calculation
	Counter     float64   // current counter (aktueller Zählerstand), eg kWh, l, m³
	UnitCounter string    // unit of current meter counter eg kWh, l, m³
	Gauge       float64   // mass flow rate per time unit  (= counter/time(h)), eg kW, l/h, m³/h
	UnitGauge   string    // unit of gauge, eg Wh, l/s, m³/h
}

func (app *App) calcGauge() {
	p := app.config.DataCollectionInterval
	for range time.Tick(p) {
		debug.DebugLog.Print("calc gauge")

		for n, m := range app.meters {
			m.Lock()
			m.Gauge = float64(m.S0.Counter-m.S0.LastCounter) / p.Hours() * m.Config.ScaleFactorCounter * m.Config.ScaleFactorGauge
			m.S0.LastCounter = m.S0.Counter
			m.TimeStamp = time.Now()
			m.Unlock()

			go app.sendMQTT(n)
		}
	}
}

func (app *App) sendMQTT(n string) {
	m, ok := app.meters[n]
	if !ok {
		return
	}

	m.Lock()
	defer m.Unlock()

	if m.Counter != m.MQTT.LastCounter || m.Gauge != m.MQTT.LastGauge {
		m.MQTT.LastCounter = m.Counter
		m.MQTT.LastGauge = m.Gauge
		m.MQTT.LastTimeStamp = m.TimeStamp

		go func(t string, r MQTTRecord) {
			debug.TraceLog.Printf("prepare mqtt message %v %v", t, r)

			b, err := json.MarshalIndent(r, "", "  ")
			if err != nil {
				debug.ErrorLog.Printf("sendMQTT marshal: %v", err)
				return
			}

			app.mqtt.C <- mqtt.Message{
				Qos:      0,
				Retained: true,
				Topic:    t,
				Payload:  b,
			}
		}(m.Config.MqttTopic,
			MQTTRecord{
				TimeStamp:   m.TimeStamp,
				Counter:     m.Counter,
				UnitCounter: m.Config.UnitCounter,
				Gauge:       m.Gauge,
				UnitGauge:   m.Config.UnitGauge,
			})
	}
}

func (app *App) loadMeasurements() (err error) {
	// if file doesn't exists, create an empty file
	fileName := app.config.DataFile
	if !tools.FileExists(fileName) {
		s := SaveMeters{}

		for name := range app.meters {
			s[name] = SavedRecord{Counter: 0, TimeStamp: time.Time{}}
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
			meter.Lock()
			meter.Counter = loadedMeter.Counter
			meter.TimeStamp = loadedMeter.TimeStamp
			meter.Unlock()
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
	debug.DebugLog.Print("saveMeasurements measurements to file")

	s := SaveMeters{}

	for name, m := range app.meters {
		m.RLock()
		s[name] = SavedRecord{Counter: m.Counter, TimeStamp: m.TimeStamp}
		m.RUnlock()
	}

	// marshal the byte slice which contains the yaml file's content into SaveMeters struct
	data, err := yaml.Marshal(&s)
	if err != nil {
		return err
	}

	if err = os.WriteFile(app.config.DataFile, data, 0o600); err != nil {
		return err
	}

	return nil
}
