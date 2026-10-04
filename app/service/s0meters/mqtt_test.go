package s0meters

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
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

// TestTelegramContract pins the payload keys and the timestamp format, which follow the
// ecoflowd telegrams; consumers such as the myhome Node-RED flows rely on them.
func TestTelegramContract(t *testing.T) {
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, CounterUnit: "Wh", GaugeUnit: "W", MqttTopic: "home/wallbox"}, 34341804),
	})

	h.mux.RLock()
	b, err := h.serializeMetricLocked("wallbox")
	h.mux.RUnlock()
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatal(err)
	}

	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"counter", "counterUnit", "gauge", "gaugeUnit", "meter", "timestamp"}
	if !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}

	if payload["meter"] != "wallbox" || payload["counter"] != float64(34341804) || payload["counterUnit"] != "Wh" {
		t.Errorf("payload = %s", b)
	}

	// RFC 3339 with offset, whole seconds - no fraction.
	ts, _ := payload["timestamp"].(string)
	if !regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(Z|[+-]\d\d:\d\d)$`).MatchString(ts) {
		t.Errorf("timestamp = %q, want RFC 3339 in whole seconds", ts)
	}
}

func TestReadingUsesLocalTimeInWholeSeconds(t *testing.T) {
	m := newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1}, 1)
	vienna, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	now := time.Date(2026, 10, 4, 22, 50, 35, 649486212, vienna)

	b, err := json.Marshal(reading("wallbox", m, now))
	if err != nil {
		t.Fatal(err)
	}
	if want := `"timestamp":"2026-10-04T22:50:35+02:00"`; !strings.Contains(string(b), want) {
		t.Errorf("payload %s does not contain %s", b, want)
	}
}
