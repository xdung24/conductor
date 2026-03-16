package monitor

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/xdung24/service-monitor/internal/models"
)

// ---------------------------------------------------------------------------
// MQTT checker
// ---------------------------------------------------------------------------

// MQTTChecker connects to an MQTT broker, subscribes to the configured topic,
// and waits for a message within the monitor timeout. It succeeds when at
// least one message is received; if HTTPKeyword is set, the message must also
// contain that string (or NOT contain it when HTTPKeywordInvert is true).
//
// Monitor field usage:
//
//	URL            — broker address, e.g. "mqtt://host:1883" or "tcp://host:1883"
//	MQTTTopic      — topic to subscribe, required
//	MQTTUsername   — optional broker username
//	MQTTPassword   — optional broker password
//	HTTPKeyword    — optional keyword to assert in the received message
//	HTTPKeywordInvert — invert: message must NOT contain the keyword
type MQTTChecker struct{}

// Check connects to the MQTT broker and waits for a message on the configured topic.
func (c *MQTTChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	brokerURL := m.URL
	if !strings.Contains(brokerURL, "://") {
		brokerURL = "mqtt://" + brokerURL
	}

	msgCh := make(chan string, 1)
	errCh := make(chan error, 1)

	//nolint:gosec // random suffix for client ID — not security-sensitive
	clientID := fmt.Sprintf("service-monitor-%08x", rand.Uint32())

	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetUsername(m.MQTTUsername).
		SetPassword(m.MQTTPassword).
		SetConnectTimeout(time.Duration(m.TimeoutSeconds) * time.Second).
		SetAutoReconnect(false).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			select {
			case errCh <- fmt.Errorf("connection lost: %w", err):
			default:
			}
		})

	client := mqtt.NewClient(opts)

	token := client.Connect()
	if wait := token.WaitTimeout(time.Duration(m.TimeoutSeconds) * time.Second); !wait {
		return Result{Status: 0, Message: "MQTT connect timeout"}
	}
	if err := token.Error(); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("connect: %v", err)}
	}
	defer client.Disconnect(250)

	topic := m.MQTTTopic
	if topic == "" {
		topic = "#"
	}

	subToken := client.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
		select {
		case msgCh <- string(msg.Payload()):
		default:
		}
	})
	if wait := subToken.WaitTimeout(time.Duration(m.TimeoutSeconds) * time.Second); !wait {
		return Result{Status: 0, Message: "MQTT subscribe timeout"}
	}
	if err := subToken.Error(); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("subscribe: %v", err)}
	}

	select {
	case <-ctx.Done():
		return Result{Status: 0, Message: "timeout waiting for MQTT message"}
	case err := <-errCh:
		return Result{Status: 0, Message: err.Error()}
	case payload := <-msgCh:
		latency := int(time.Since(start).Milliseconds())
		if m.HTTPKeyword != "" {
			contains := strings.Contains(payload, m.HTTPKeyword)
			if contains == m.HTTPKeywordInvert {
				msg := fmt.Sprintf("keyword %q", m.HTTPKeyword)
				if m.HTTPKeywordInvert {
					msg += " found but should be absent"
				} else {
					msg += " not found in message"
				}
				return Result{Status: 0, LatencyMs: latency, Message: msg}
			}
		}
		return Result{
			Status:    1,
			LatencyMs: latency,
			Message:   fmt.Sprintf("topic: %s; message: %s", topic, payload),
		}
	}
}
