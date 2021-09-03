package meter

import (
	"s0counter/pkg/app/config"
	"s0counter/pkg/raspberry"
	"sync"
	"time"
)

type S0 struct {
	Tick          uint64    // s0 ticks overall
	TimeStamp     time.Time // time of the last s0 pulse
	LastTimeStamp time.Time // time of the penultimate s0 pulse
}

type Meter struct {
	sync.RWMutex
	LineHandler raspberry.Pin
	Config      config.MeterConfig
	//	TimeStamp   time.Time // timestamp of last gauge calculation
	//	Counter     float64   // current counter (aktueller Zählerstand), eg kWh, l, m³
	//	Gauge       float64   // mass flow rate per time unit  (= counter/t), e.g. kW, l/h, m³/h
	S0 S0
}

func New() map[string]*Meter {
	return map[string]*Meter{}
}
