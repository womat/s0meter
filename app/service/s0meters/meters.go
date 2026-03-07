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
//	    counterPulsesPerUnit:    1000,
//	    CounterUnit:     "kWh",
//	    GaugeScale:     1.0,
//	    CounterPrecision:       2,
//	    GaugePrecision:       2,
//	    GaugeUnit:       "kW",
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

// Handler manages all registered meters, MQTT, and data persistence.
type Handler struct {
	mux    sync.RWMutex
	logger *slog.Logger
	meters map[string]*MeterInstance
}

// MeterConfig defines the configuration of a single meter.
type MeterConfig struct {
	Gpio       int `yaml:"gpio"`       // GPIO pin for pulse input
	BounceTime int `yaml:"bounceTime"` // Debounce in ms

	CounterUnit          string  `yaml:"counterUnit"`          // unit of counter value (e.g. kWh, Wh)
	CounterPulsesPerUnit float64 `yaml:"counterPulsesPerUnit"` // number of pulses per counterUnit (e.g. 1000 imp/kWh)
	CounterPrecision     int     `yaml:"counterPrecision"`     // decimal places for counter

	GaugeUnit      string  `yaml:"gaugeUnit"`      // unit of gauge value (e.g. W, kW)
	GaugeScale     float64 `yaml:"gaugeScale"`     // scale factor for gauge (1=W, 0.001=kW)
	GaugePrecision int     `yaml:"gaugePrecision"` // decimal places for gauge

	MqttTopic    string `yaml:"mqttTopic"`    // MQTT topic for this meter
	MqttRetained bool   `yaml:"mqttRetained"` // MQTT retained flag
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
	CounterUnit string    `json:"counterUnit"` // Counter unit
	Gauge       float64   `json:"gauge"`       // Flow rate
	GaugeUnit   string    `json:"gaugeUnit"`   // Gauge unit
}

// New initializes the meter manager with the provided configuration.
func New() *Handler {
	return &Handler{
		meters: make(map[string]*MeterInstance),
	}
}

// Close shuts down all meters, disconnects MQTT.
func (h *Handler) Close() error {
	h.mux.Lock()
	defer h.mux.Unlock()

	var errs error
	for _, m := range h.meters {
		errs = errors.Join(errs, m.Meter.Close())
	}
	return errs
}

// RegisterMeter adds a new S0 meter and initializes its pulse handler.
func (h *Handler) RegisterMeter(ctx context.Context, name string, cfg MeterConfig) error {
	meter, err := pulsecounter.New(ctx, cfg.Gpio, time.Duration(cfg.BounceTime)*time.Millisecond)
	if err != nil {
		return err
	}

	h.mux.Lock()
	h.meters[name] = &MeterInstance{
		Config: cfg,
		Meter:  meter,
	}
	h.mux.Unlock()
	return nil
}

// GetMeterAll returns the current reading of all meters.
func (h *Handler) GetMeterAll() map[string]MeterData {
	h.mux.RLock()
	defer h.mux.RUnlock()

	now := time.Now()
	data := make(map[string]MeterData, len(h.meters))
	for name, m := range h.meters {
		data[name] = MeterData{
			TimeStamp:   now,
			Counter:     calcCounter(m),
			CounterUnit: m.Config.CounterUnit,
			Gauge:       calcGauge(m),
			GaugeUnit:   m.Config.GaugeUnit,
		}
	}
	return data
}

// GetMeter returns the reading of a specific meter.
func (h *Handler) GetMeter(name string) (MeterData, error) {
	h.mux.RLock()
	m, ok := h.meters[name]
	h.mux.RUnlock()

	if !ok {
		return MeterData{}, fmt.Errorf("meter %s not found", name)
	}

	return MeterData{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		CounterUnit: m.Config.CounterUnit,
		Gauge:       calcGauge(m),
		GaugeUnit:   m.Config.GaugeUnit,
	}, nil
}

// IsReady returns true if all optional services are connected.
// This can be used for Kubernetes-style Readiness checks (/ready endpoint).
func (h *Handler) IsReady() bool {
	return true
}

// Validate checks the MeterConfig for invalid or missing values.
func (c *MeterConfig) Validate() error {
	if c.Gpio <= 0 {
		return fmt.Errorf("gpio pin must be greater than 0, got %v", c.Gpio)
	}
	if c.BounceTime < 0 {
		return fmt.Errorf("bounceTime must be >= 0, got %v", c.BounceTime)
	}
	if c.CounterPulsesPerUnit <= 0 {
		return fmt.Errorf("counterPulsesPerUnit must be greater than 0, got %v", c.CounterPulsesPerUnit)
	}
	if c.CounterPrecision < 0 {
		return fmt.Errorf("counter precision must be >= 0, got %d", c.CounterPrecision)
	}
	if c.GaugeScale == 0 {
		return fmt.Errorf("gaugeScale must not be 0")
	}
	if c.GaugePrecision < 0 {
		return fmt.Errorf("gauge precision must be >= 0, got %d", c.GaugePrecision)
	}
	return nil
}

// calcGauge computes the flow rate based on the last two pulses.
func calcGauge(m *MeterInstance) float64 {
	c := m.Meter.GetCounter()

	if c.LastTimeStamp.IsZero() || c.TimeStamp.IsZero() {
		return 0
	}

	dt := c.TimeStamp.Sub(c.LastTimeStamp)
	if elapsed := time.Since(c.TimeStamp); elapsed > dt {
		dt = elapsed
	}

	if dt <= 0 {
		return 0
	}

	val := 3600 / dt.Seconds() * m.Config.GaugeScale
	return round(val, m.Config.GaugePrecision)
}

// calcCounter computes the total meter value from pulses.
func calcCounter(m *MeterInstance) float64 {
	c := m.Meter.GetCounter()
	if m.Config.CounterPulsesPerUnit == 0 {
		return 0
	}
	return round(float64(c.Pulses)/m.Config.CounterPulsesPerUnit, m.Config.CounterPrecision)
}

// round rounds a float to a given precision.
func round(num float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(num*p) / p
}
