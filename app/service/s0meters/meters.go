// Package s0meters manages multiple S0 pulse meters and publishes their data via MQTT.
//
// It supports real and emulated GPIO meters, tracks counters, calculates gauge values,
// and provides periodic data backup to YAML.
//
// Example:
//
//	config := meters.Config{
//	    DataFile:               "meter_data.yaml",
//	    BackupInterval:         300,
//	    DataCollectionInterval: 60,
//	    MqttRetained:           true,
//	    MqttTopic:              "s0meters",
//	}
//
//	handler := meters.New(config)
//
//	handler.AddMeter("power", meters.MeterConfig{
//	    Gpio:            17,
//	    BounceTime:      100,
//	    CounterConstant: 1000,
//	    UnitCounter:     "kWh",
//	    ScaleFactor:     1.0,
//	    Precision:       2,
//	    UnitGauge:       "kW",
//	    MqttTopic:       "meters/power",
//	})
//
//	handler.Connect("tcp://mqtt.example.com:1883")
//	defer handler.Close()
package s0meters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"s0meter/pkg/pulsecounter"
	"sync"
	"time"
)

// Config defines global settings for the meter manager.
type Config struct {
	DataFile               string // Path to YAML file for meter backup
	BackupInterval         int    // Interval for saving data (seconds)
	DataCollectionInterval int    // Interval for publishing gauge values (seconds)
}

// Handler manages all registered meters, MQTT, and data persistence.
type Handler struct {
	sync.RWMutex
	config Config
	meters map[string]*MeterInstance
}

// MeterConfig defines the configuration of a single meter.
type MeterConfig struct {
	Gpio         int     `yaml:"gpio"`         // GPIO pin for pulse input
	BounceTime   int     `yaml:"bounceTime"`   // Debounce in ms
	TicksPerUnit float64 `yaml:"ticksPerUnit"` // Pulses per unit (calibration)
	UnitCounter  string  `yaml:"unitCounter"`  // Unit of counter (e.g., kWh)
	ScaleFactor  float64 `yaml:"scaleFactor"`  // Scale factor for gauge
	Precision    int     `yaml:"precision"`    // Decimal precision
	UnitGauge    string  `yaml:"unitGauge"`    // Unit of gauge (e.g., kW)
	MqttTopic    string  `yaml:"mqttTopic"`    // MQTT topic for this meter
	MqttRetained bool    `yaml:"mqttRetained"` // MQTT retained flag
}

// MeterInstance holds a registered meter and its pulse handler.
type MeterInstance struct {
	Config MeterConfig
	Meter  *pulsecounter.Handler
}

// MeterData represents the calculated counter and gauge readings.
type MeterData struct {
	TimeStamp   time.Time `json:"timeStamp"`   // Timestamp of reading
	Counter     float64   `json:"counter"`     // Total meter value
	UnitCounter string    `json:"unitCounter"` // Counter unit
	Gauge       float64   `json:"gauge"`       // Flow rate
	UnitGauge   string    `json:"unitGauge"`   // Gauge unit
}

// New initializes the meter manager with the provided configuration.
func New(config Config) *Handler {
	return &Handler{
		config: config,
		meters: make(map[string]*MeterInstance),
	}
}

// Close shuts down all meters, disconnects MQTT, and persists data.
func (h *Handler) Close() error {
	h.RLock()
	defer h.RUnlock()

	var err error
	for _, m := range h.meters {
		slog.Debug("Closing meter", "name", m.Config.Gpio)
		err = errors.Join(err, m.Meter.Close())
	}

	slog.Debug("Saving meter data")
	err = errors.Join(err, h.saveMeterData())
	return err
}

// RegisterMeter adds a new S0 meter and initializes its pulse handler.
func (h *Handler) RegisterMeter(ctx context.Context, name string, cfg MeterConfig) error {

	slog.Info("Initializing meter", "name", name, "gpio", cfg.Gpio)

	meter, err := pulsecounter.New(ctx, cfg.Gpio, time.Duration(cfg.BounceTime)*time.Millisecond)
	if err != nil {
		slog.Error("Failed to initialize meter", "name", name, "error", err)
		return err
	}

	h.Lock()
	h.meters[name] = &MeterInstance{
		Config: cfg,
		Meter:  meter,
	}
	h.Unlock()
	return nil
}

// GetMeterData returns the current reading of all meters.
func (h *Handler) GetMeterData() map[string]MeterData {
	h.RLock()
	defer h.RUnlock()

	now := time.Now()
	data := make(map[string]MeterData, len(h.meters))
	for name, m := range h.meters {
		data[name] = MeterData{
			TimeStamp:   now,
			Counter:     calcCounter(m),
			UnitCounter: m.Config.UnitCounter,
			Gauge:       calcGauge(m),
			UnitGauge:   m.Config.UnitGauge,
		}
	}
	return data
}

// GetMeter returns the reading of a specific meter.
func (h *Handler) GetMeter(name string) (MeterData, error) {
	h.RLock()
	m, ok := h.meters[name]
	h.RUnlock()

	if !ok {
		return MeterData{}, fmt.Errorf("meter %s not found", name)
	}

	return MeterData{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		UnitCounter: m.Config.UnitCounter,
		Gauge:       calcGauge(m),
		UnitGauge:   m.Config.UnitGauge,
	}, nil
}

// IsReady returns true if all optional services are connected.
// This can be used for Kubernetes-style Readiness checks (/ready endpoint).
func (h *Handler) IsReady() bool {
	return true
}

// calcGauge computes the flow rate based on the last two pulses.
func calcGauge(m *MeterInstance) float64 {
	c := m.Meter.GetCounter()
	dt := c.TimeStamp.Sub(c.LastTimeStamp)

	if time.Since(c.TimeStamp) > dt {
		dt = time.Since(c.LastTimeStamp)
	}

	if dt.Seconds() <= 0 || m.Config.TicksPerUnit == 0 {
		return 0
	}

	val := float64(c.Ticks) * 3600 / (dt.Seconds() * m.Config.TicksPerUnit) * m.Config.ScaleFactor
	return round(val, m.Config.Precision)
}

// calcCounter computes the total meter value from pulse ticks.
func calcCounter(m *MeterInstance) float64 {
	c := m.Meter.GetCounter()
	if m.Config.TicksPerUnit == 0 {
		return 0
	}
	return float64(c.Ticks) / m.Config.TicksPerUnit
}

// round rounds a float to a given precision.
func round(num float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(num*p) / p
}
