package s0meters

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/womat/mqtt"
)

// Publisher is the broker connection the loop publishes to. *mqtt.Handler implements it; the
// tests use a fake, because womat/mqtt offers none.
type Publisher interface {
	// Publish sends one message and reports whether the broker took it.
	Publish(msg mqtt.Message) error
	// IsConnectionOpen reports whether a broker connection is open right now; it is false
	// while a reconnect is pending, when a QoS 0 message would be dropped silently.
	IsConnectionOpen() bool
}

// RunPeriodicPublish runs the publishing loop. It blocks, so the caller runs it in a goroutine it
// can wait for.
//
// A meter is published as soon as a new S0 pulse has been counted, and in any case once per
// heartbeat. The trigger compares the raw pulse count, not the rounded counter, so a pulse is
// published even when it does not change the counter at its configured precision. Between pulses
// only the gauge decays (gaugeAt stretches the interval to the time since the last pulse), which
// is not worth a message of its own — the heartbeat carries it. Meters without an MQTT topic are
// not published.
//
// The loop wakes every minInterval to look for new pulses, so minInterval is both the worst-case
// delay of a pulse and the shortest spacing between two messages of the same meter. That spacing
// is what keeps a fast pulsing meter from flooding the broker: a wallbox charging at 11 kW with
// one pulse per Wh produces roughly three pulses per second. A minInterval of zero disables the
// change trigger and leaves the plain heartbeat.
//
// Ticks are skipped while pub.IsConnectionOpen is false, so a disconnect costs neither a failed
// publish per meter nor a warning per tick.
//
// The loop stops when the provided context is cancelled.
func (h *Handler) RunPeriodicPublish(ctx context.Context, heartbeat, minInterval time.Duration, pub Publisher) {
	tick := minInterval
	if tick <= 0 || tick > heartbeat {
		tick = heartbeat
	}

	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	due := heartbeatDue(heartbeat, tick)

	// Owned by this loop alone, so it needs no lock of its own.
	state := make(map[string]publishState)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping periodic MQTT publishing")
			return
		case <-ticker.C:
			if !pub.IsConnectionOpen() {
				slog.Debug("Skipping MQTT publish, broker not connected")
				continue
			}
			h.publishDue(pub, state, due)
		}
	}
}

// heartbeatDue returns the age at which a meter's last message counts as due for the heartbeat.
//
// The loop wakes every tick, and every wake-up is a little late by a varying amount. Comparing
// the age with heartbeat itself would skip a tick whenever this wake-up was less late than the
// one that sent the last message, so the heartbeat would come one tick later, a full heartbeat
// later when tick equals heartbeat. Half a tick of slack absorbs that jitter, while a message is
// still never repeated before heartbeat minus half a tick.
func heartbeatDue(heartbeat, tick time.Duration) time.Duration {
	return heartbeat - tick/2
}

// publishState records what was last delivered for one meter, so the loop can tell a new pulse
// from the gauge merely decaying, and knows when the next heartbeat is due.
type publishState struct {
	pulses uint64
	at     time.Time
}

// pendingMsg couples a serialized meter reading with the meter it came from, so publish errors
// can still be reported per meter and the state updated after the lock has been released.
type pendingMsg struct {
	name   string
	pulses uint64
	msg    mqtt.Message
}

// publishDue sends the meters that advanced or whose heartbeat is due, and records what went out.
func (h *Handler) publishDue(pub Publisher, state map[string]publishState, heartbeat time.Duration) {
	now := time.Now()
	for _, p := range publishPending(pub, h.collectPending(state, heartbeat, now)) {
		state[p.name] = publishState{pulses: p.pulses, at: now}
	}
}

// collectPending serializes the meters that are due to be published.
//
// Meters without an MQTT topic are never collected. A meter is collected when a pulse was counted
// since the last message in state or when that message is older than heartbeat; a meter not in
// state, as in a nil state, always is. The recorded pulse count and the payload come from the
// same counter snapshot, so a pulse counted in between is neither lost nor sent twice. The
// caller publishes after the lock has been released.
func (h *Handler) collectPending(state map[string]publishState, heartbeat time.Duration, now time.Time) []pendingMsg {
	h.mux.RLock()
	defer h.mux.RUnlock()

	pending := make([]pendingMsg, 0, len(h.meters))
	for name, meterInstance := range h.meters {
		if meterInstance.Config.MqttTopic == "" {
			continue
		}

		c := meterInstance.Meter.GetCounter()
		if prev, sent := state[name]; sent && c.Pulses == prev.pulses && now.Sub(prev.at) < heartbeat {
			continue
		}

		b, err := json.Marshal(reading(name, c, meterInstance.Config, now))
		if err != nil {
			slog.Warn("Failed to serialize metric", "meter", name, "error", err)
			continue
		}

		pending = append(pending, pendingMsg{
			name:   name,
			pulses: c.Pulses,
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
func publishPending(pub Publisher, pending []pendingMsg) []pendingMsg {
	sent := make([]pendingMsg, 0, len(pending))

	for _, p := range pending {
		if err := pub.Publish(p.msg); err != nil {
			slog.Warn("Failed to publish MQTT message", "meter", p.name, "topic", p.msg.Topic, "error", err)
			continue
		}
		sent = append(sent, p)
	}

	return sent
}
