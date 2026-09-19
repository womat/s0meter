package s0meters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/womat/golib/mqtt"
)

// StartPeriodicPublish runs the publishing loop in a separate goroutine.
//
// A meter is published as soon as its counter advances, i.e. as soon as a new S0 pulse has been
// counted, and in any case once per heartbeat. Between pulses only the gauge decays (calcGauge
// stretches the interval to the time since the last pulse), which is not worth a message of its
// own — the heartbeat carries it.
//
// The loop wakes every minInterval to look for new pulses, so minInterval is both the worst-case
// delay of a pulse and the shortest spacing between two messages of the same meter. That spacing
// is what keeps a fast pulsing meter from flooding the broker: a wallbox charging at 11 kW with
// one pulse per Wh produces roughly three pulses per second. A minInterval of zero disables the
// change trigger and leaves the plain heartbeat.
//
// isConnected reports whether the broker connection is currently established. Ticks are skipped
// while it returns false, because Publish() blocks until its timeout expires when the client is
// disconnected. Passing nil disables the check.
//
// The loop stops when the provided context is cancelled.
func (h *Handler) StartPeriodicPublish(ctx context.Context, heartbeat, minInterval time.Duration, mqttHandler *mqtt.Handler, isConnected func() bool) {
	tick := minInterval
	if tick <= 0 || tick > heartbeat {
		tick = heartbeat
	}

	ticker := time.NewTicker(tick)

	go func() {
		defer ticker.Stop()

		// Owned by this goroutine alone, so it needs no lock of its own.
		state := make(map[string]publishState)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopping periodic MQTT publishing")
				return
			case <-ticker.C:
				if isConnected != nil && !isConnected() {
					slog.Debug("Skipping MQTT publish, broker not connected")
					continue
				}
				h.publishDue(mqttHandler, state, heartbeat)
			}
		}
	}()
}

// publishState records what was last delivered for one meter, so the loop can tell a new pulse
// from the gauge merely decaying, and knows when the next heartbeat is due.
type publishState struct {
	counter float64
	at      time.Time
}

// pendingMsg couples a serialized meter reading with the meter it came from, so publish errors
// can still be reported per meter and the state updated after the lock has been released.
type pendingMsg struct {
	name    string
	counter float64
	msg     mqtt.Message
}

// PublishAllMetrics sends the current reading of every meter, regardless of change or heartbeat.
func (h *Handler) PublishAllMetrics(mqttHandler *mqtt.Handler) {
	if mqttHandler == nil {
		return
	}

	publishPending(mqttHandler, h.collectPending(nil, 0, time.Now()))
}

// publishDue sends the meters that advanced or whose heartbeat is due, and records what went out.
func (h *Handler) publishDue(mqttHandler *mqtt.Handler, state map[string]publishState, heartbeat time.Duration) {
	if mqttHandler == nil {
		return
	}

	now := time.Now()
	for _, p := range publishPending(mqttHandler, h.collectPending(state, heartbeat, now)) {
		state[p.name] = publishState{counter: p.counter, at: now}
	}
}

// collectPending serializes the meters that are due to be published.
//
// A nil state collects every meter; otherwise a meter is collected when its counter advanced since
// the last message or when that message is older than heartbeat. Serialization happens under RLock
// so that the caller can publish without holding it.
func (h *Handler) collectPending(state map[string]publishState, heartbeat time.Duration, now time.Time) []pendingMsg {
	h.mux.RLock()
	defer h.mux.RUnlock()

	pending := make([]pendingMsg, 0, len(h.meters))
	for name, meterInstance := range h.meters {
		counter := calcCounter(meterInstance)

		if state != nil {
			if prev, sent := state[name]; sent && counter == prev.counter && now.Sub(prev.at) < heartbeat {
				continue
			}
		}

		b, err := h.serializeMetricLocked(name)
		if err != nil {
			slog.Warn("Failed to serialize metric", "meter", name, "error", err)
			continue
		}

		pending = append(pending, pendingMsg{
			name:    name,
			counter: counter,
			msg: mqtt.Message{
				Topic:    meterInstance.Config.MqttTopic,
				Payload:  b,
				Qos:      0,
				Retained: meterInstance.Config.MqttRetained,
			},
		})
	}

	return pending
}

// publishPending sends each message and returns those the broker accepted. A message that failed
// is left out, so its meter is retried on the next tick instead of being recorded as delivered.
func publishPending(mqttHandler *mqtt.Handler, pending []pendingMsg) []pendingMsg {
	sent := make([]pendingMsg, 0, len(pending))

	for _, p := range pending {
		if err := mqttHandler.Publish(p.msg); err != nil {
			slog.Warn("Failed to publish MQTT message", "meter", p.name, "topic", p.msg.Topic, "error", err)
			continue
		}
		sent = append(sent, p)
	}

	return sent
}

// SerializeMetric — public, acquires own lock
func (h *Handler) SerializeMetric(name string) ([]byte, error) {
	h.mux.RLock()
	defer h.mux.RUnlock()
	return h.serializeMetricLocked(name)
}

// serializeMetricLocked — caller must hold RLock
func (h *Handler) serializeMetricLocked(name string) ([]byte, error) {
	m, ok := h.meters[name]
	if !ok {
		return nil, fmt.Errorf("meter %s not registered", name)
	}
	payload := MeterData{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		CounterUnit: m.Config.CounterUnit,
		Gauge:       calcGauge(m),
		GaugeUnit:   m.Config.GaugeUnit,
	}
	return json.Marshal(payload)
}
