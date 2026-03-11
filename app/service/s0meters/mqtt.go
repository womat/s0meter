package s0meters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/womat/golib/mqtt"
)

// StartPeriodicPublish runs a periodic publishing loop in a separate goroutine.
//
// This function publishes all meter readings every DataCollectionInterval seconds.
// The loop stops when the provided context is cancelled.
func (h *Handler) StartPeriodicPublish(ctx context.Context, interval time.Duration, mqttHandler *mqtt.Handler) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopping periodic MQTT publishing")
				return
			case <-ticker.C:
				h.PublishAllMetrics(mqttHandler)
			}
		}
	}()
}

// PublishAllMetrics sends all metrics to the given MQTT handler
func (h *Handler) PublishAllMetrics(mqttHandler *mqtt.Handler) {
	if mqttHandler == nil {
		return
	}

	h.mux.RLock()
	defer h.mux.RUnlock()

	for name, meterInstance := range h.meters {
		b, err := h.serializeMetricLocked(name)
		if err != nil {
			slog.Warn("Failed to serialize metric", "meter", name, "error", err)
			continue
		}

		msg := mqtt.Message{
			Topic:    meterInstance.Config.MqttTopic,
			Payload:  b,
			Qos:      0,
			Retained: meterInstance.Config.MqttRetained,
		}

		if err = mqttHandler.Publish(msg); err != nil {
			slog.Warn("Failed to publish MQTT message", "meter", name, "topic", msg.Topic, "error", err)
		}
	}
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
