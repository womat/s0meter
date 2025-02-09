package mqtt

import (
	mqttlib "github.com/eclipse/paho.mqtt.golang"
	"log/slog"
)

// quiesce is the specified number of milliseconds to wait for existing work to be completed.
const (
	quiesce = 250
)

// Handler contains the handler of the mqtt broker
type Handler struct {
	handler mqttlib.Client
}

// Message contains the properties of the mqtt message
type Message struct {
	Topic    string
	Payload  []byte
	Qos      byte
	Retained bool
}

// New generate a new mqtt broker client
func New() *Handler {
	return &Handler{}
}

// Connect connecting to the mqtt broker
func (m *Handler) Connect(broker string) error {

	opts := mqttlib.NewClientOptions().AddBroker(broker)
	m.handler = mqttlib.NewClient(opts)
	return m.ReConnect()
}

// ReConnect reconnects to the defined mqtt broker
func (m *Handler) ReConnect() error {
	t := m.handler.Connect()
	<-t.Done()
	return t.Error()
}

// Disconnect will end the connection to the broker
func (m *Handler) Disconnect() error {

	m.handler.Disconnect(quiesce)
	return nil
}

// Publish sends a message to the mqtt broker. If the connection is lost, it will try to reconnect.
// If the connection can't be established, it will return an error.
// The message is sent asynchronously. If the message can't be sent, it will be logged.
func (m *Handler) Publish(msg Message) error {
	if !m.handler.IsConnected() {
		slog.Debug("MQTT broker isn't connected, reconnect it")

		if err := m.ReConnect(); err != nil {
			x := m.handler.OptionsReader()
			slog.Error("Can't reconnect to MQTT broker", "broker", x.Servers()[0].Host, "error", err)
			return err
		}
	}

	slog.Debug("Publishing MQTT message", "topic", msg.Topic, "payload", string(msg.Payload))
	t := m.handler.Publish(msg.Topic, msg.Qos, msg.Retained, msg.Payload)

	// The asynchronous nature of this library makes it easy to forget to check for errors.
	// Consider using a go routine to log these
	go func() {
		<-t.Done()
		slog.Debug("MQTT message published", "topic", msg.Topic, "payload", string(msg.Payload))

		if err := t.Error(); err != nil {
			slog.Error("Publishing MQTT message failed", "topic", msg.Topic, "error", err)
		}
	}()

	return nil
}
