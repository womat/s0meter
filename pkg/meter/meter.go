package meter

import (
	"s0counter/pkg/app/config"
	"s0counter/pkg/raspberry"
	"sync"
	"time"
)

type S0 struct {
	Counter     int64     // s0 counter since program start
	LastCounter int64     // s0 counter at the last average calculation
	TimeStamp   time.Time // time of last s0 pulse
}

type Meter struct {
	sync.RWMutex
	LineHandler      *raspberry.Line    `json:"-"`
	Config           config.MeterConfig `json:"-"`
	TimeStamp        time.Time          // time of last throughput calculation
	MeterReading     float64            // current meter reading (aktueller Zählerstand), eg kWh, l, m³
	UnitMeterReading string             // unit of current meter reading eg kWh, l, m³
	Flow             float64            // mass flow rate per time unit  (= flow/time(h)), eg kW, l/h, m³/h
	UnitFlow         string             // unit of mass flow rate per time unit, eg Wh, l/s, m³/h
	S0               S0                 `json:"-"`
}

func New() map[string]*Meter {
	return map[string]*Meter{}
}
