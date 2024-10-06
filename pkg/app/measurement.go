package app

import (
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/womat/tools"

	"s0counter/pkg/meter"
	"s0counter/pkg/mqtt"
)

type SavedRecord struct {
	Ticks     uint64    `yaml:"ticks"`     // current s0 ticks
	Counter   float64   `yaml:"counter"`   // current meter counter (aktueller Zählerstand), e.g., kWh, l, m³ >> isn't necessary anymore, compatibility reason
	TimeStamp time.Time `yaml:"timestamp"` // time of last s0 pulse
}
type SaveMeters map[string]SavedRecord

type MQTTRecord struct {
	TimeStamp   time.Time // timestamp of last gauge calculation
	Counter     float64   // current counter (aktueller Zählerstand), eg kWh, l, m³
	UnitCounter string    // unit of current meter counter e.g., kWh, l, m³
	Gauge       float64   // mass flow rate per time unit  (= counter/time(h)), e.g. kW, l/h, m³/h
	UnitGauge   string    // unit of gauge, eg Wh, l/s, m³/h
}

// calcGauge periodically calculates the gauge- and counter-values for each meter
// based on the configured data collection interval and sends the results over MQTT.
func (app *App) calcGauge() {
	p := app.config.DataCollectionInterval
	for range time.Tick(p) {
		for _, m := range app.meters {
			go app.publish(m)
		}
	}
}

// publish sends the current counter and gauge values of a meter to the MQTT broker.
// The values are sent as a JSON string.
// The topic is defined in the configuration file.
func (app *App) publish(m *meter.Meter) error {

	m.RLock()

	payload := MQTTRecord{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		UnitCounter: m.Config.UnitCounter,
		Gauge:       calcGauge(m),
		UnitGauge:   m.Config.UnitGauge,
	}
	topic := m.Config.MqttTopic
	defer m.RUnlock()

	slog.Debug("prepare mqtt message", "topic", topic, "record", payload)

	b, err := json.Marshal(payload)
	if err != nil {
		slog.Error("sendMQTT marshal", "error", err)
		return err
	}

	msg := mqtt.Message{
		Qos:      0,
		Retained: app.config.MQTT.Retained,
		Topic:    topic,
		Payload:  b,
	}

	err = app.mqtt.Publish(msg)
	if err != nil {
		slog.Error("sendMQTT publish", "error", err)
	}

	return err
}

// loadMeasurements loads the current counter and gauge values of each meter from a YAML file.
// If the file doesn't exist, it will be created with default values.

func (app *App) loadMeasurements() (err error) {
	// if the file doesn't exist, create an empty file
	fileName := app.config.DataFile
	if !tools.FileExists(fileName) {
		s := SaveMeters{}

		for name := range app.meters {
			s[name] = SavedRecord{Counter: 0, TimeStamp: time.Time{}}
		}

		// marshal the byte slice which contains the YAML file's content into SaveMeters struct
		var data []byte
		data, err = yaml.Marshal(&s)
		if err != nil {
			return
		}

		if err = os.WriteFile(fileName, data, 0o600); err != nil {
			return
		}
	}

	// read the YAML file as a byte array.
	data, err := os.ReadFile(fileName)
	if err != nil {
		return
	}

	// unmarshal the byte slice which contains the YAML file's content into SaveMeters struct
	s := SaveMeters{}
	if err = yaml.Unmarshal(data, &s); err != nil {
		return
	}

	for name, loadedMeter := range s {
		if m, ok := app.meters[name]; ok {
			m.Lock()
			m.S0.TimeStamp = loadedMeter.TimeStamp
			m.S0.Tick = loadedMeter.Ticks
			m.Unlock()
		}
	}

	return
}

// backupMeasurements periodically saves the current counter and gauge values of each meter to a YAML file.
// The interval is defined in the configuration file.
func (app *App) backupMeasurements() {
	for range time.Tick(app.config.BackupInterval) {
		_ = app.saveMeasurements()
	}
}

// saveMeasurements saves the current counter and gauge values of each meter to a YAML file.
// The file is created if it doesn't exist. If the file exists, it will be overwritten.
func (app *App) saveMeasurements() error {
	slog.Debug("saveMeasurements", "file", app.config.DataFile)

	s := SaveMeters{}

	for name, m := range app.meters {
		m.RLock()
		s[name] = SavedRecord{Ticks: m.S0.Tick, Counter: calcCounter(m), TimeStamp: m.S0.TimeStamp}
		m.RUnlock()
	}

	// marshal the byte slice which contains the YAML file's content into SaveMeters struct
	data, err := yaml.Marshal(&s)
	if err != nil {
		return err
	}

	if err = os.WriteFile(app.config.DataFile, data, 0o600); err != nil {
		return err
	}

	return nil
}

// calcGauge calculates the current gauge value of a meter
// by dividing the number of ticks by the counter-constant
// and multiplying it by the scale factor.
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
	return toFixed(f, m.Config.Precision)
}

// calcCounter calculates the current counter-value of a meter
// by dividing the number of ticks by the counter-constant
func calcCounter(m *meter.Meter) (f float64) {
	f = float64(m.S0.Tick) / m.Config.CounterConstant
	return f
}

func toFixed(num float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(num*p) / p
}
