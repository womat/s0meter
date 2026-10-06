package s0meters

import (
	"math"
	"testing"
	"time"

	"github.com/womat/s0meter/pkg/pulsecounter"
)

func TestGaugeAt(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	cfg := MeterConfig{GaugeScale: 1, GaugePrecision: 1}

	tests := []struct {
		name     string
		last, ts time.Time
		want     float64
	}{
		{"one pulse per second", now.Add(-2 * time.Second), now.Add(-time.Second), 3600},
		{"decays once pulses stop", now.Add(-11 * time.Second), now.Add(-10 * time.Second), 360},
		{"first pulse only", time.Time{}, now, 0},
		{"no pulse yet", time.Time{}, time.Time{}, 0},
		{"interval not positive", now.Add(time.Hour), now.Add(-100 * time.Millisecond), 0},
		{"equal timestamps", now.Add(-time.Second), now.Add(-time.Second), 0},
		// Timestamps ahead of the clock, as after a reboot without RTC: the interval
		// still applies and no spike from the near-zero or negative elapsed time.
		{"timestamps in the future", now.Add(59 * time.Second), now.Add(time.Minute), 3600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := pulsecounter.Counter{Pulses: 5, LastTimeStamp: tt.last, TimeStamp: tt.ts}
			if got := gaugeAt(c, cfg, now); got != tt.want {
				t.Errorf("gaugeAt = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGaugeAtScales(t *testing.T) {
	now := time.Now()
	// One pulse per Wh, 1 s apart: 3600 W, shown in kW.
	c := pulsecounter.Counter{LastTimeStamp: now.Add(-2 * time.Second), TimeStamp: now.Add(-time.Second)}
	if got := gaugeAt(c, MeterConfig{GaugeScale: 0.001, GaugePrecision: 2}, now); got != 3.6 {
		t.Errorf("kW gauge = %v, want 3.6", got)
	}
}

func TestStatusOf(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	cfg := MeterConfig{Gpio: 17, CounterUnit: "l", CounterPulsesPerUnit: 1, GaugeUnit: "l/h", GaugeScale: 1}

	s := statusOf(pulsecounter.Counter{Pulses: 42}, cfg, 3, now)
	if s.LastPulse != nil || s.LastPulseAgeSeconds != nil {
		t.Errorf("no pulse since start: lastPulse %v, age %v, want both nil", s.LastPulse, s.LastPulseAgeSeconds)
	}
	if s.Gpio != 17 || s.Pulses != 42 || s.DroppedEvents != 3 {
		t.Errorf("statusOf = %+v, want config and counts passed through", s)
	}
	if want := (Display{Counter: 42, CounterUnit: "l", GaugeUnit: "l/h"}); s.Display != want {
		t.Errorf("display without display units = %+v, want the configured units %+v", s.Display, want)
	}

	ts := now.Add(-12500 * time.Millisecond)
	s = statusOf(pulsecounter.Counter{Pulses: 43, TimeStamp: ts}, cfg, 0, now)
	if s.LastPulse == nil || !s.LastPulse.Equal(ts.Truncate(time.Second)) {
		t.Errorf("lastPulse = %v, want %v", s.LastPulse, ts.Truncate(time.Second))
	}
	if s.LastPulseAgeSeconds == nil || *s.LastPulseAgeSeconds != 12.5 {
		t.Errorf("age = %v, want 12.5", s.LastPulseAgeSeconds)
	}

	// Clock set back below the last pulse: no negative age.
	s = statusOf(pulsecounter.Counter{TimeStamp: now.Add(time.Minute)}, cfg, 0, now)
	if *s.LastPulseAgeSeconds != 0 {
		t.Errorf("age of a pulse ahead of the clock = %v, want 0", *s.LastPulseAgeSeconds)
	}
}

func TestDisplayOf(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	// One pulse per Wh, the last two 1 s apart: 3600 W.
	c := pulsecounter.Counter{Pulses: 34341881, LastTimeStamp: now.Add(-time.Second), TimeStamp: now}
	cfg := MeterConfig{
		CounterUnit: "Wh", CounterPulsesPerUnit: 1, CounterPrecision: 0,
		GaugeUnit: "W", GaugeScale: 1, GaugePrecision: 2,
		DisplayUnit: "kWh", DisplayGaugeUnit: "kW",
	}

	want := Display{
		Counter: 34341.881, CounterUnit: "kWh", CounterPrecision: 3,
		Gauge: 3.6, GaugeUnit: "kW", GaugePrecision: 5,
	}
	if got := displayOf(c, cfg, now); got != want {
		t.Errorf("displayOf = %+v, want %+v", got, want)
	}

	// Water: 1 pulse = 1 l, shown in m³ like the register of a water meter.
	water := MeterConfig{CounterUnit: "l", CounterPulsesPerUnit: 1, GaugeUnit: "l/h", GaugeScale: 1, DisplayUnit: "m³"}
	if got := displayOf(pulsecounter.Counter{Pulses: 1024317}, water, now); got.Counter != 1024.317 || got.CounterPrecision != 3 {
		t.Errorf("water display = %+v, want 1024.317 m³ with 3 places", got)
	}
}

func TestCounterOf(t *testing.T) {
	cfg := MeterConfig{CounterPulsesPerUnit: 1000, CounterPrecision: 3}
	if got := counterOf(pulsecounter.Counter{Pulses: 105459}, cfg); got != 105.459 {
		t.Errorf("counterOf = %v, want 105.459", got)
	}
}

func TestRound(t *testing.T) {
	if got := round(1.23456, 2); got != 1.23 {
		t.Errorf("round = %v, want 1.23", got)
	}
	if got := round(2.5, 0); got != 3 {
		t.Errorf("round = %v, want 3", got)
	}
	if got := round(1.5, maxPrecision); math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("round at maxPrecision = %v", got)
	}
}

func TestMeterConfigValidate(t *testing.T) {
	valid := MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}

	invalid := map[string]func(*MeterConfig){
		"gpio 0":                  func(c *MeterConfig) { c.Gpio = 0 },
		"gpio 1":                  func(c *MeterConfig) { c.Gpio = 1 },
		"gpio 28":                 func(c *MeterConfig) { c.Gpio = 28 },
		"negative debounce":       func(c *MeterConfig) { c.DebounceTime = -time.Millisecond },
		"zero pulses per unit":    func(c *MeterConfig) { c.CounterPulsesPerUnit = 0 },
		"infinite pulses":         func(c *MeterConfig) { c.CounterPulsesPerUnit = math.Inf(1) },
		"NaN gauge scale":         func(c *MeterConfig) { c.GaugeScale = math.NaN() },
		"negative gauge scale":    func(c *MeterConfig) { c.GaugeScale = -1 },
		"counter precision 16":    func(c *MeterConfig) { c.CounterPrecision = 16 },
		"negative gauge precison": func(c *MeterConfig) { c.GaugePrecision = -1 },
	}
	for name, mutate := range invalid {
		t.Run(name, func(t *testing.T) {
			c := valid
			mutate(&c)
			if err := c.Validate(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
