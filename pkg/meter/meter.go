package meter

import (
	"log/slog"
	"sync"
	"time"

	"s0counter/pkg/app/config"
	"s0counter/pkg/rpi"
)

// S0 is the struct to store the s0 pulse data
type S0 struct {
	Tick          uint64    // s0 ticks overall
	TimeStamp     time.Time // time of the last s0 pulse
	LastTimeStamp time.Time // time of the penultimate s0 pulse
}

// Meter is the struct to store the meter data
type Meter struct {
	sync.RWMutex
	LineHandler *rpi.Port
	Config      config.MeterConfig
	//	TimeStamp   time.Time // timestamp of last gauge calculation
	//	Counter     float64   // current counter (aktueller Zählerstand), e.g., kWh, l, m³
	//	Gauge       float64   // mass flow rate per time unit (= counter/t), e.g., kW, l/h, m³/h
	S0 S0
}

// New initializes a new meter
func New(c config.MeterConfig) *Meter {
	return &Meter{
		RWMutex:     sync.RWMutex{},
		LineHandler: nil,
		Config:      c,
		S0: S0{
			Tick:          0,
			TimeStamp:     time.Time{},
			LastTimeStamp: time.Time{},
		},
	}
}

// EventHandler is the event handler for the s0 pulse.
// It counts the s0 pulses and stores the time of the last pulse.
// The rpi package calls this event handler.
func (m *Meter) EventHandler(event rpi.Event) {

	if event.Type == rpi.RisingEdge {
		slog.Debug("receive a rising impulse", "pin", m.Config.Gpio, "timestamp", event.Timestamp)
		m.Lock()
		m.S0.LastTimeStamp = m.S0.TimeStamp
		m.S0.TimeStamp = event.Timestamp
		m.S0.Tick++
		m.Unlock()
	}
}
