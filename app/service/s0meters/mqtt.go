package s0meters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"s0meter/pkg/mqtt"
	"time"
)

// StartPeriodicPublish runs a periodic publishing loop in a separate goroutine.
//
// This function publishes all meter readings every DataCollectionInterval seconds.
// The loop stops when the provided context is cancelled.
func (h *Handler) StartPeriodicPublish(ctx context.Context, mqttHandler *mqtt.Handler) {
	interval := time.Duration(h.config.DataCollectionInterval) * time.Second
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		slog.Info("Started periodic MQTT publishing", "interval", interval)

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
	h.RLock()
	defer h.RUnlock()

	if mqttHandler == nil {
		return
	}
	for name, meterInstance := range h.meters {
		b, err := h.SerializeMetric(name)
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

// SerializeMetric returns the JSON payload for a meter
func (h *Handler) SerializeMetric(name string) ([]byte, error) {
	h.RLock()
	m, ok := h.meters[name]
	h.RUnlock()
	if !ok {
		return nil, fmt.Errorf("meter %s not registered", name)
	}

	payload := MeterData{
		TimeStamp:   time.Now(),
		Counter:     calcCounter(m),
		UnitCounter: m.Config.UnitCounter,
		Gauge:       calcGauge(m),
		UnitGauge:   m.Config.UnitGauge,
	}

	return json.Marshal(payload)
}
