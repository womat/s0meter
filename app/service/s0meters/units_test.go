package s0meters

import (
	"math"
	"testing"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		from, to string
		table    map[string]unit
		factor   float64
		prec     int // display precision for a configured precision of 0
	}{
		{"Wh", "kWh", counterUnits, 0.001, 3},
		{"kWh", "MWh", counterUnits, 0.001, 3},
		{"kWh", "Wh", counterUnits, 1000, 0},
		{"l", "m³", counterUnits, 0.001, 3},
		{"l", "m3", counterUnits, 0.001, 3},
		{"min", "h", counterUnits, 1.0 / 60, 2},
		{"Wh", "Wh", counterUnits, 1, 0},
		{"W", "kW", gaugeUnits, 0.001, 3},
		{"l/h", "m³/h", gaugeUnits, 0.001, 3},
		{"l/s", "l/h", gaugeUnits, 3600, 0},
		{"l/min", "l/h", gaugeUnits, 60, 0},
	}
	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			f, err := convert(tt.from, tt.to, tt.table)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(f-tt.factor) > 1e-12*tt.factor {
				t.Errorf("factor = %v, want %v", f, tt.factor)
			}
			if p := displayPrecision(0, f); p != tt.prec {
				t.Errorf("displayPrecision(0) = %d, want %d", p, tt.prec)
			}
		})
	}
}

func TestConvertRejects(t *testing.T) {
	tests := []struct {
		from, to string
		table    map[string]unit
	}{
		{"Wh", "m³", counterUnits},   // different quantities
		{"W", "l/h", gaugeUnits},     // different quantities
		{"imp", "kWh", counterUnits}, // unknown source unit
		{"Wh", "kwh", counterUnits},  // unknown target unit (case matters)
		{"kWh", "kW", counterUnits},  // a gauge unit is no counter unit
	}
	for _, tt := range tests {
		if _, err := convert(tt.from, tt.to, tt.table); err == nil {
			t.Errorf("convert(%s, %s) succeeded, want an error", tt.from, tt.to)
		}
	}
}

func TestDisplayPrecisionClamps(t *testing.T) {
	if p := displayPrecision(2, 1e6); p != 0 {
		t.Errorf("MWh with 2 places shown in Wh: got %d places, want 0", p)
	}
	if p := displayPrecision(maxPrecision, 0.001); p != maxPrecision {
		t.Errorf("precision above maxPrecision: got %d, want %d", p, maxPrecision)
	}
}
