package mqtt

import (
	mqttlib "github.com/eclipse/paho.mqtt.golang"
	"github.com/womat/debug"
)

type Handler struct {
	handler mqttlib.Client
	C       chan Message
}

type Message struct {
	Topic   string
	Payload []byte
}

func NewMqtt() *Handler {
	return &Handler{
		C: make(chan Message),
	}
}

func (m *Handler) Connect(c string) error {
	if c == "" {
		return nil
	}

	opts := mqttlib.NewClientOptions().AddBroker(c)
	m.handler = mqttlib.NewClient(opts)
	err := m.ReConnect()
	return err
}

func (m *Handler) ReConnect() error {
	token := m.handler.Connect()
	token.Wait()
	err := token.Error()
	return err
}

func (m *Handler) Disconnect() error {
	if m.handler == nil {
		return nil
	}

	m.handler.Disconnect(250)
	return nil
}

func (m *Handler) Service() {
	for data := range m.C {
		data := data

		go func() {
			if m.handler == nil {
				return
			}

			if !m.handler.IsConnected() {
				if err := m.ReConnect(); err != nil {
					debug.ErrorLog.Printf("can't reconnect to mqtt broker /%v", err)

					return
				}
			}

			token := m.handler.Publish(data.Topic, 0, false, data.Payload) //nolint:wsl
			token.Wait()

			if err := token.Error(); err != nil {
				debug.ErrorLog.Printf("publishing topic %v: %v", data.Topic, err)
			}
		}()
	}
}
