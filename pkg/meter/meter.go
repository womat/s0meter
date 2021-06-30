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

type MQTT struct {
	LastCounter   float64   // counter of last mqtt message
	LastGauge     float64   // gauge of last mqtt message
	LastTimeStamp time.Time // time of last mqtt message
}

type Meter struct {
	sync.RWMutex
	LineHandler raspberry.Pin
	Config      config.MeterConfig
	TimeStamp   time.Time // timestamp of last gauge calculation
	Counter     float64   // current counter (aktueller Zählerstand), eg kWh, l, m³
	Gauge       float64   // mass flow rate per time unit  (= counter/time(h)), eg kW, l/h, m³/h
	LastGauge   float64
	LastCounter float64
	S0          S0
	MQTT        MQTT
}

func New() map[string]*Meter {
	return map[string]*Meter{}
}
