package app

import (
	"encoding/json"
	"os"
	"s0counter/pkg/meter"
	"s0counter/pkg/mqtt"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/womat/debug"
	"github.com/womat/tools"
)

type SavedRecord struct {
	Ticks     uint64    `yaml:"ticks"`     // current s0 ticks
	Counter   float64   `yaml:"counter"`   // current meter counter (aktueller Zählerstand), eg kWh, l, m³ >> is not needed anymore, compatibility reason
	TimeStamp time.Time `yaml:"timestamp"` // time of last s0 pulse
}
type SaveMeters map[string]SavedRecord

type MQTTRecord struct {
	TimeStamp   time.Time // timestamp of last gauge calculation
	Counter     float64   // current counter (aktueller Zählerstand), eg kWh, l, m³
	UnitCounter string    // unit of current meter counter e.g. kWh, l, m³
	Gauge       float64   // mass flow rate per time unit  (= counter/time(h)), e.g. kW, l/h, m³/h
	UnitGauge   string    // unit of gauge, eg Wh, l/s, m³/h
}

func (app *App) calcGauge() {
	p := app.config.DataCollectionInterval
	for range time.Tick(p) {
		for n := range app.meters {
			go app.sendMQTT(n)
		}
	}
}

func (app *App) sendMQTT(n string) {
	m, ok := app.meters[n]
	if !ok {
		return
	}

	m.RLock()
	defer m.RUnlock()

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
			TimeStamp:   time.Now(),
			Counter:     calcCounter(m),
			UnitCounter: m.Config.UnitCounter,
			Gauge:       calcGauge(m),
			UnitGauge:   m.Config.UnitGauge,
		})
}

func (app *App) loadMeasurements() (err error) {
	// if file doesn't exist, create an empty file
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
		if m, ok := app.meters[name]; ok {
			m.Lock()
			m.S0.LastTimeStamp = loadedMeter.TimeStamp
			m.S0.TimeStamp = time.Now()

			// compatibility: calculate s0 Tick
			if loadedMeter.Ticks == 0 && loadedMeter.Counter > 0 {
				loadedMeter.Ticks = uint64(loadedMeter.Counter * m.Config.CounterConstant)
			}

			m.S0.Tick = loadedMeter.Ticks
			m.Unlock()
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
		s[name] = SavedRecord{Ticks: m.S0.Tick, Counter: calcCounter(m), TimeStamp: m.S0.TimeStamp}
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

func calcGauge(m *meter.Meter) (f float64) {
	dt := m.S0.TimeStamp.Sub(m.S0.LastTimeStamp) // duration between last two ticks

	// if duration between "now and last tick" is greater than the duration between "last two ticks"
	// increase duration between "last two ticks" to duration between "now and the penultimate tick".
	// this ensures that the value of the gauge value changes if no new pulse has been received
	if time.Since(m.S0.TimeStamp) > dt {
		dt = time.Since(m.S0.LastTimeStamp)
	}

	// P = 3600 / (t * Cz) (Zählerkonstante in Imp/kWh)
	f = 3600 / (dt.Seconds() * m.Config.CounterConstant) * m.Config.ScaleFactor
	return f
}

func calcCounter(m *meter.Meter) (f float64) {
	f = float64(m.S0.Tick) / m.Config.CounterConstant
	return f
}
