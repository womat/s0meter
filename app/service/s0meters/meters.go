// Package s0meters is the domain layer of s0meter: it turns the pulses of several S0 meters
// into readings and publishes and persists them.
//
// A Handler holds the registered meters. For each it derives the counter (pulses divided by
// the meter constant) and the gauge (pulses per hour from the last interval, scaled), see
// counterOf and gaugeAt. backup.go keeps the pulse counts in a YAML file, written
// atomically; mqtt.go publishes a meter on every new pulse and on a heartbeat.
//
// Typical use, as wired up by package app:
//
//	saved, err := s0meters.ReadMeterData(file)
//	h := s0meters.New()
//	err = h.RegisterMeter("wallbox", cfg, saved["wallbox"])
//	go h.RunPeriodicBackup(ctx, time.Minute, file)
//	go h.RunPeriodicPublish(ctx, time.Minute, 2*time.Second, mqttHandler, mqttHandler.IsConnectionOpen)
//	defer h.Close()
package s0meters

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/womat/s0meter/pkg/pulsecounter"
)

// Handler manages all registered meters. It is safe for concurrent use.
type Handler struct {
	mux    sync.RWMutex
	meters map[string]*MeterInstance
}

// MeterConfig defines the configuration of a single meter.
type MeterConfig struct {
	Gpio         int           `yaml:"gpio"`         // GPIO pin for pulse input
	DebounceTime time.Duration `yaml:"debounceTime"` // Debounce as Go duration string (e.g. 10ms)

	CounterUnit          string  `yaml:"counterUnit"`          // unit of counter value (e.g. kWh, Wh)
	CounterPulsesPerUnit float64 `yaml:"counterPulsesPerUnit"` // number of pulses per counterUnit (e.g. 1000 imp/kWh)
	CounterPrecision     int     `yaml:"counterPrecision"`     // decimal places for counter

	GaugeUnit      string  `yaml:"gaugeUnit"`      // unit of gauge value (e.g. W, kW)
	GaugeScale     float64 `yaml:"gaugeScale"`     // scale factor for gauge (1=W, 0.001=kW)
	GaugePrecision int     `yaml:"gaugePrecision"` // decimal places for gauge

	MqttTopic    string `yaml:"mqttTopic"`    // MQTT topic for this meter
	MqttRetained bool   `yaml:"mqttRetained"` // broker keeps the last message of this topic (default: false)

	// Units the web UI shows counter and gauge in (empty = CounterUnit/GaugeUnit). Display only,
	// the telegram keeps the configured units; see units.go for the known conversions.
	DisplayUnit      string `yaml:"displayUnit"`
	DisplayGaugeUnit string `yaml:"displayGaugeUnit"`
}

// MeterInstance holds a registered meter and its pulse handler.
type MeterInstance struct {
	Config MeterConfig
	Meter  *pulsecounter.Handler
}

// MeterData is one reading of a meter: the telegram published via MQTT and returned by the API.
//
// The keys follow the telegrams of ecoflowd (myhome/KONZEPT-ECOFLOW.md): camelCase, "timestamp"
// as one word, the device named in every telegram. TimeStamp is the time of the reading in
// local time with offset, whole seconds (RFC 3339, e.g. 2026-10-04T22:50:35+02:00). For an S0
// meter that is also the measuring time, because every pulse is counted the moment it arrives.
type MeterData struct {
	Meter       string    `json:"meter"`       // Meter name from the config, like ecoflowd's "sn"
	TimeStamp   time.Time `json:"timestamp"`   // Time of the reading, local time, whole seconds
	Counter     float64   `json:"counter"`     // Total meter value in CounterUnit
	CounterUnit string    `json:"counterUnit"` // Counter unit
	Gauge       float64   `json:"gauge"`       // Flow rate in GaugeUnit
	GaugeUnit   string    `json:"gaugeUnit"`   // Gauge unit
}

// New initializes the meter manager with the provided configuration.
func New() *Handler {
	return &Handler{
		meters: make(map[string]*MeterInstance),
	}
}

// Close stops counting on all meters and releases their GPIO lines. The counters stay readable.
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
// Counting continues from pulses, the value restored by ReadMeterData.
func (h *Handler) RegisterMeter(name string, cfg MeterConfig, pulses uint64) error {
	meter, err := pulsecounter.New(cfg.Gpio, cfg.DebounceTime, pulses)
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
		data[name] = reading(name, m, now)
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

	return reading(name, m, time.Now()), nil
}

// reading builds the MeterData of meter name at now. Counter and gauge come from the same
// counter snapshot, so they always describe the same state of the meter.
func reading(name string, m *MeterInstance, now time.Time) MeterData {
	c := m.Meter.GetCounter()
	return MeterData{
		Meter:       name,
		TimeStamp:   now.Truncate(time.Second),
		Counter:     counterOf(c, m.Config),
		CounterUnit: m.Config.CounterUnit,
		Gauge:       gaugeAt(c, m.Config, now),
		GaugeUnit:   m.Config.GaugeUnit,
	}
}

// MeterStatus is the diagnostic state of one meter, reported by /health. It is not part of
// the telegram: MQTT and /meters carry MeterData only.
//
// LastPulse and LastPulseAgeSeconds are null until the first pulse after a start or reload,
// because the pulse timestamps are not restored from the data file. The age is computed here
// rather than by the client, whose clock may disagree with that of a Pi without RTC.
type MeterStatus struct {
	Gpio                int        `json:"gpio"`                // GPIO the meter is wired to (BCM numbering)
	Pulses              uint64     `json:"pulses"`              // Raw pulse count, including restored pulses
	LastPulse           *time.Time `json:"lastPulse"`           // Time of the last pulse, local time, whole seconds
	LastPulseAgeSeconds *float64   `json:"lastPulseAgeSeconds"` // Seconds since the last pulse, one decimal place
	DroppedEvents       uint64     `json:"droppedEvents"`       // GPIO events lost since start or reload, see pulsecounter
	Display             Display    `json:"display"`             // Counter and gauge in the display units
}

// Display is a reading converted to the display units of a meter (DisplayUnit,
// DisplayGaugeUnit), for the web UI. Without display units it equals the telegram's values.
// The precisions are raised or lowered with the unit, so the resolution stays the same.
type Display struct {
	Counter          float64 `json:"counter"`          // Counter in CounterUnit
	CounterUnit      string  `json:"counterUnit"`      // DisplayUnit, or the configured counterUnit
	CounterPrecision int     `json:"counterPrecision"` // Decimal places of Counter
	Gauge            float64 `json:"gauge"`            // Gauge in GaugeUnit
	GaugeUnit        string  `json:"gaugeUnit"`        // DisplayGaugeUnit, or the configured gaugeUnit
	GaugePrecision   int     `json:"gaugePrecision"`   // Decimal places of Gauge
}

// Status returns the diagnostic state of all meters at now.
func (h *Handler) Status(now time.Time) map[string]MeterStatus {
	h.mux.RLock()
	defer h.mux.RUnlock()

	status := make(map[string]MeterStatus, len(h.meters))
	for name, m := range h.meters {
		status[name] = statusOf(m.Meter.GetCounter(), m.Config, m.Meter.DroppedEvents(), now)
	}
	return status
}

// statusOf builds the MeterStatus of a counter snapshot at now. A last pulse that lies ahead of
// now, as when the clock is set back, is reported with age 0.
func statusOf(c pulsecounter.Counter, cfg MeterConfig, dropped uint64, now time.Time) MeterStatus {
	s := MeterStatus{
		Gpio:          cfg.Gpio,
		Pulses:        c.Pulses,
		DroppedEvents: dropped,
		Display:       displayOf(c, cfg, now),
	}
	if c.TimeStamp.IsZero() {
		return s
	}

	last := c.TimeStamp.Truncate(time.Second)
	age := round(max(now.Sub(c.TimeStamp), 0).Seconds(), 1)
	s.LastPulse, s.LastPulseAgeSeconds = &last, &age
	return s
}

// displayOf converts a counter snapshot at now to the display units of cfg. Both values are
// converted before rounding, so no precision is lost to the rounding of the configured unit.
// Validate has checked the conversions; should one fail anyway, the configured unit is kept.
func displayOf(c pulsecounter.Counter, cfg MeterConfig, now time.Time) Display {
	d := Display{
		CounterUnit:      cfg.CounterUnit,
		CounterPrecision: cfg.CounterPrecision,
		GaugeUnit:        cfg.GaugeUnit,
		GaugePrecision:   cfg.GaugePrecision,
	}

	counterFactor, gaugeFactor := 1.0, 1.0
	if cfg.DisplayUnit != "" {
		if f, err := convert(cfg.CounterUnit, cfg.DisplayUnit, counterUnits); err == nil {
			counterFactor, d.CounterUnit = f, cfg.DisplayUnit
			d.CounterPrecision = displayPrecision(cfg.CounterPrecision, f)
		}
	}
	if cfg.DisplayGaugeUnit != "" {
		if f, err := convert(cfg.GaugeUnit, cfg.DisplayGaugeUnit, gaugeUnits); err == nil {
			gaugeFactor, d.GaugeUnit = f, cfg.DisplayGaugeUnit
			d.GaugePrecision = displayPrecision(cfg.GaugePrecision, f)
		}
	}

	if cfg.CounterPulsesPerUnit != 0 {
		d.Counter = round(float64(c.Pulses)/cfg.CounterPulsesPerUnit*counterFactor, d.CounterPrecision)
	}
	d.Gauge = round(rateAt(c, cfg, now)*gaugeFactor, d.GaugePrecision)
	return d
}

// Range of the usable GPIOs (BCM numbering) on the 40-pin header. GPIO0 and GPIO1 are on the
// header too, but reserved for the ID EEPROM of HAT boards.
const (
	minGpio = 2
	maxGpio = 27
)

// maxPrecision is the most decimal places a float64 reading can meaningfully carry; beyond it
// round's power of ten overflows to Inf, and the reading becomes NaN.
const maxPrecision = 15

// Validate checks the MeterConfig for invalid or missing values.
func (c *MeterConfig) Validate() error {
	switch {
	case c.Gpio == 0 || c.Gpio == 1:
		return fmt.Errorf("gpio %d is reserved for the HAT ID EEPROM, use %d-%d (BCM numbering)", c.Gpio, minGpio, maxGpio)
	case c.Gpio < minGpio || c.Gpio > maxGpio:
		return fmt.Errorf("gpio %d is not on the 40-pin header, use %d-%d (BCM numbering)", c.Gpio, minGpio, maxGpio)
	}
	if c.DebounceTime < 0 {
		return fmt.Errorf("debounceTime must be non-negative, got %v", c.DebounceTime)
	}
	if !isPositiveFinite(c.CounterPulsesPerUnit) {
		return fmt.Errorf("counterPulsesPerUnit must be a positive number, got %v", c.CounterPulsesPerUnit)
	}
	if c.CounterPrecision < 0 || c.CounterPrecision > maxPrecision {
		return fmt.Errorf("counterPrecision must be 0-%d, got %d", maxPrecision, c.CounterPrecision)
	}
	if !isPositiveFinite(c.GaugeScale) {
		return fmt.Errorf("gaugeScale must be a positive number, got %v", c.GaugeScale)
	}
	if c.GaugePrecision < 0 || c.GaugePrecision > maxPrecision {
		return fmt.Errorf("gaugePrecision must be 0-%d, got %d", maxPrecision, c.GaugePrecision)
	}
	if c.DisplayUnit != "" {
		if _, err := convert(c.CounterUnit, c.DisplayUnit, counterUnits); err != nil {
			return fmt.Errorf("displayUnit: %w", err)
		}
	}
	if c.DisplayGaugeUnit != "" {
		if _, err := convert(c.GaugeUnit, c.DisplayGaugeUnit, gaugeUnits); err != nil {
			return fmt.Errorf("displayGaugeUnit: %w", err)
		}
	}
	return nil
}

// isPositiveFinite reports whether f is greater than 0 and neither Inf nor NaN.
func isPositiveFinite(f float64) bool {
	return f > 0 && !math.IsInf(f, 0)
}

// gaugeAt computes the flow rate of a counter snapshot at now, from the last two pulses.
//
// The result is pulses per hour times GaugeScale; CounterPulsesPerUnit plays no part. The
// interval is stretched to the time since the last pulse when that is longer, so the rate
// decays toward 0 once pulses stop. An interval that is not positive yields 0 rather than
// being replaced by that elapsed time, which right after a pulse is close to zero and would
// turn into a huge spike - the case when the two timestamps come from clocks that disagree.
func gaugeAt(c pulsecounter.Counter, cfg MeterConfig, now time.Time) float64 {
	return round(rateAt(c, cfg, now), cfg.GaugePrecision)
}

// rateAt is gaugeAt without the rounding, for conversions to a display unit.
func rateAt(c pulsecounter.Counter, cfg MeterConfig, now time.Time) float64 {
	if c.LastTimeStamp.IsZero() || c.TimeStamp.IsZero() {
		return 0
	}

	interval := c.TimeStamp.Sub(c.LastTimeStamp)
	if interval <= 0 {
		return 0
	}

	dt := max(interval, now.Sub(c.TimeStamp))

	return 3600 / dt.Seconds() * cfg.GaugeScale
}

// counterOf computes the total meter value of a counter snapshot: pulses divided by the meter
// constant, rounded to CounterPrecision.
func counterOf(c pulsecounter.Counter, cfg MeterConfig) float64 {
	if cfg.CounterPulsesPerUnit == 0 {
		return 0
	}
	return round(float64(c.Pulses)/cfg.CounterPulsesPerUnit, cfg.CounterPrecision)
}

// round rounds a float to a given precision.
func round(num float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(num*p) / p
}
