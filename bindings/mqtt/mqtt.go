// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/dapr/pkg/logger"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

// MQTT allows sending and receiving data to/from an MQTT broker
type MQTT struct {
	metadata *mqttMetadata
	client   mqtt.Client

	logger logger.Logger
}

// Metadata is the MQTT config
type mqttMetadata struct {
	URL   string `json:"url"`
	Topic string `json:"topic"`
}

// NewMQTT returns a new MQTT instance
func NewMQTT(logger logger.Logger) *MQTT {
	return &MQTT{logger: logger}
}

// Init does MQTT connection parsing
func (m *MQTT) Init(metadata bindings.Metadata) error {
	mqttMeta, err := m.getMQTTMetadata(metadata)
	if err != nil {
		return err
	}

	m.metadata = mqttMeta
	if m.metadata.URL == "" {
		return errors.New("MQTT Error: URL required")
	}

	if m.metadata.Topic == "" {
		return errors.New("MQTT error: topic required")
	}

	uri, err := url.Parse(m.metadata.URL)
	if err != nil {
		return err
	}
	client, err := m.connect(uuid.New().String(), uri)
	if err != nil {
		return err
	}
	m.client = client

	return nil
}

func (m *MQTT) getMQTTMetadata(metadata bindings.Metadata) (*mqttMetadata, error) {
	b, err := json.Marshal(metadata.Properties)
	if err != nil {
		return nil, err
	}

	var mMetadata mqttMetadata
	err = json.Unmarshal(b, &mMetadata)
	if err != nil {
		return nil, err
	}

	return &mMetadata, nil
}

func (m *MQTT) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{bindings.CreateOperation}
}

func (m *MQTT) Invoke(req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	m.client.Publish(m.metadata.Topic, 0, false, string(req.Data))
	m.client.Disconnect(0)

	return nil, nil
}

func (m *MQTT) Read(handler func(*bindings.ReadResponse) ([]byte, error)) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	m.client.Subscribe(m.metadata.Topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		handler(&bindings.ReadResponse{
			Data: msg.Payload(),
		})
	})
	<-c

	return nil
}

func (m *MQTT) connect(clientID string, uri *url.URL) (mqtt.Client, error) {
	opts := m.createClientOptions(clientID, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		return nil, err
	}

	return client, nil
}

func (m *MQTT) createClientOptions(clientID string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientID)

	return opts
}
