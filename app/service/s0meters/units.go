package s0meters

// Display units: the web UI may show a meter in another unit than it counts in, e.g. a Wh
// counter in kWh or a litre counter in m³. Only the display changes; the telegram (MQTT and
// /meters) keeps counterUnit and gaugeUnit. A conversion is allowed only between units of the
// same quantity listed here, so a typo or Wh -> m³ is a start-up error instead of a wrong value.

import (
	"fmt"
	"math"
)

// unit is one entry of a conversion table: the physical quantity it measures and its size in
// the base unit of that quantity.
type unit struct {
	quantity string
	factor   float64
}

// counterUnits are the known units of a counter (an amount).
var counterUnits = map[string]unit{
	"Wh":  {"energy", 1},
	"kWh": {"energy", 1e3},
	"MWh": {"energy", 1e6},
	"l":   {"volume", 1},
	"m³":  {"volume", 1e3},
	"m3":  {"volume", 1e3},
	"s":   {"time", 1},
	"min": {"time", 60},
	"h":   {"time", 3600},
}

// gaugeUnits are the known units of a gauge (an amount per time).
var gaugeUnits = map[string]unit{
	"W":     {"power", 1},
	"kW":    {"power", 1e3},
	"MW":    {"power", 1e6},
	"l/s":   {"flow", 1},
	"l/min": {"flow", 1.0 / 60},
	"l/h":   {"flow", 1.0 / 3600},
	"m³/h":  {"flow", 1.0 / 3.6},
	"m3/h":  {"flow", 1.0 / 3.6},
}

// convert returns the factor that turns a value in unit from into unit to, looked up in table.
// It fails when either unit is unknown or the two measure different quantities.
func convert(from, to string, table map[string]unit) (float64, error) {
	f, ok := table[from]
	if !ok {
		return 0, fmt.Errorf("unit %q has no known conversion", from)
	}
	t, ok := table[to]
	if !ok {
		return 0, fmt.Errorf("unit %q has no known conversion", to)
	}
	if f.quantity != t.quantity {
		return 0, fmt.Errorf("cannot convert %s (%s) to %s (%s)", from, f.quantity, to, t.quantity)
	}
	return f.factor / t.factor, nil
}

// displayPrecision shifts precision by the order of magnitude of factor, so the converted value
// keeps the resolution of the original: Wh with 0 places becomes kWh with 3.
func displayPrecision(precision int, factor float64) int {
	p := precision + int(math.Round(-math.Log10(factor)))
	return min(max(p, 0), maxPrecision)
}
