package s0meters

import (
	"testing"
	"time"
)

func TestCollectPending(t *testing.T) {
	const heartbeat = time.Minute
	now := time.Now()

	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, MqttTopic: "home/wallbox"}, 10),
		"silent":  newMeter(t, MeterConfig{Gpio: 5, CounterPulsesPerUnit: 1, GaugeScale: 1}, 10),
	})

	pending := h.collectPending(map[string]publishState{}, heartbeat, now)
	if len(pending) != 1 || pending[0].name != "wallbox" || pending[0].pulses != 10 {
		t.Fatalf("first tick = %+v, want wallbox only, without the meter that has no topic", pending)
	}
	if pending[0].msg.Topic != "home/wallbox" {
		t.Errorf("topic = %q", pending[0].msg.Topic)
	}

	state := map[string]publishState{"wallbox": {pulses: 10, at: now}}

	if got := h.collectPending(state, heartbeat, now.Add(time.Second)); len(got) != 0 {
		t.Errorf("no new pulse within the heartbeat should publish nothing, got %+v", got)
	}
	if got := h.collectPending(state, heartbeat, now.Add(heartbeat)); len(got) != 1 {
		t.Errorf("due heartbeat should publish, got %+v", got)
	}
	if got := h.collectPending(nil, 0, now); len(got) != 1 {
		t.Errorf("nil state collects every meter with a topic, got %+v", got)
	}
}

func TestCollectPendingTriggersOnPulsesNotRoundedCounter(t *testing.T) {
	now := time.Now()
	// 1000 pulses per unit at precision 0: pulse 11 leaves the rounded counter at 0.
	h := handlerWith(map[string]*MeterInstance{
		"water": newMeter(t, MeterConfig{Gpio: 5, CounterPulsesPerUnit: 1000, GaugeScale: 1, MqttTopic: "home/water"}, 11),
	})
	state := map[string]publishState{"water": {pulses: 10, at: now}}

	if got := h.collectPending(state, time.Minute, now.Add(time.Second)); len(got) != 1 {
		t.Errorf("a new pulse should publish although the rounded counter did not change, got %+v", got)
	}
}
