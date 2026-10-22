package s0meters

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/womat/mqtt"
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

// A tick that wakes up a little earlier than the one that sent the last message must still send
// the heartbeat, otherwise it slips by a whole tick.
func TestHeartbeatToleratesTickJitter(t *testing.T) {
	now := time.Now()
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, MqttTopic: "home/wallbox"}, 10),
	})
	state := map[string]publishState{"wallbox": {pulses: 10, at: now}}

	for _, tick := range []time.Duration{2 * time.Second, time.Minute} {
		due := heartbeatDue(time.Minute, tick)
		if got := h.collectPending(state, due, now.Add(time.Minute-time.Millisecond)); len(got) != 1 {
			t.Errorf("tick %v: heartbeat 1ms early was skipped", tick)
		}
		if got := h.collectPending(state, due, now.Add(time.Minute-tick)); len(got) != 0 {
			t.Errorf("tick %v: heartbeat one tick early was sent", tick)
		}
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
	// The Node-RED flows read gauge as a number.
	if _, ok := payload["gauge"].(float64); !ok || payload["gaugeUnit"] != "W" {
		t.Errorf("gauge %v (%T), gaugeUnit %v; want a number in W", payload["gauge"], payload["gauge"], payload["gaugeUnit"])
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

// A message the broker did not take is not recorded, so the meter is sent again on the next
// tick instead of waiting for the heartbeat.
func TestFailedPublishIsRetried(t *testing.T) {
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, MqttTopic: "home/wallbox"}, 10),
	})
	state := map[string]publishState{}

	// A handler without a client fails every publish.
	h.publishDue(&mqtt.Handler{}, state, time.Minute)

	if len(state) != 0 {
		t.Errorf("state = %+v after a failed publish, want nothing recorded", state)
	}
	if got := h.collectPending(state, time.Minute, time.Now()); len(got) != 1 {
		t.Errorf("next tick collects %d meters, want the one that failed", len(got))
	}
}

// fakePublisher records the messages it is given and whether the Handler's lock was held at the
// time, which would block every API request for as long as the broker takes to acknowledge.
type fakePublisher struct {
	h *Handler

	mu         sync.Mutex
	msgs       []mqtt.Message
	lockedOnce bool
}

func (f *fakePublisher) Publish(msg mqtt.Message) error {
	free := f.h.mux.TryLock()
	if free {
		f.h.mux.Unlock()
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, msg)
	f.lockedOnce = f.lockedOnce || !free
	return nil
}

func (f *fakePublisher) sent() []mqtt.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.msgs)
}

// Publishing happens without the Handler's lock, with the topic, retain flag and QoS 0 of each
// meter, and is recorded so the next tick does not repeat it.
func TestPublishOutsideTheLock(t *testing.T) {
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, MqttTopic: "home/wallbox", MqttRetained: true}, 10),
		"water":   newMeter(t, MeterConfig{Gpio: 5, CounterPulsesPerUnit: 1, GaugeScale: 1, MqttTopic: "home/water"}, 20),
	})
	pub := &fakePublisher{h: h}
	state := map[string]publishState{}

	h.publishDue(pub, state, time.Minute)

	msgs := pub.sent()
	if len(msgs) != 2 {
		t.Fatalf("published %d messages, want 2", len(msgs))
	}
	if pub.lockedOnce {
		t.Error("Publish ran while the Handler's lock was held")
	}
	for _, m := range msgs {
		wantRetained := m.Topic == "home/wallbox"
		if m.Retained != wantRetained || m.Qos != 0 {
			t.Errorf("%s: retained %v, qos %d; want retained %v, qos 0", m.Topic, m.Retained, m.Qos, wantRetained)
		}
	}
	if state["wallbox"].pulses != 10 || state["water"].pulses != 20 {
		t.Errorf("state = %+v, want both meters recorded", state)
	}

	h.publishDue(pub, state, time.Minute)
	if got := len(pub.sent()); got != 2 {
		t.Errorf("a second tick without new pulses published again, %d messages in total", got)
	}
}

// While the broker connection is not open the loop publishes nothing; once it is, the next tick
// publishes.
func TestPublishLoopWaitsForTheConnection(t *testing.T) {
	// The emulated pin is created outside the bubble; only the loop runs inside it.
	h := handlerWith(map[string]*MeterInstance{
		"wallbox": newMeter(t, MeterConfig{Gpio: 17, CounterPulsesPerUnit: 1, GaugeScale: 1, MqttTopic: "home/wallbox"}, 10),
	})

	synctest.Test(t, func(t *testing.T) {
		pub := &fakePublisher{h: h}
		var connected atomic.Bool
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			h.RunPeriodicPublish(ctx, time.Minute, time.Second, pub, connected.Load)
			close(done)
		}()

		// Half a tick off the ticker, so sleep and tick never wake at the same instant.
		time.Sleep(5*time.Second + 500*time.Millisecond)
		synctest.Wait()
		if got := len(pub.sent()); got != 0 {
			t.Fatalf("published %d messages without a connection", got)
		}

		connected.Store(true)
		time.Sleep(time.Second)
		synctest.Wait()
		if got := len(pub.sent()); got != 1 {
			t.Errorf("published %d messages on the first connected tick, want 1", got)
		}

		cancel()
		<-done
	})
}
