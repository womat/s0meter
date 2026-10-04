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
