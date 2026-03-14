package notifier

import (
	"context"
	"fmt"
	"log"
)

// Event holds the data passed to every notification provider when a monitor
// changes state.
type Event struct {
	MonitorID   int64
	MonitorName string
	MonitorURL  string
	Status      int // 1=up, 0=down
	LatencyMs   int
	Message     string // HTTP status / error text
}

// StatusText returns "UP" or "DOWN".
func (e Event) StatusText() string {
	if e.Status == 1 {
		return "UP"
	}
	return "DOWN"
}

// Provider is implemented by every notification backend.
type Provider interface {
	// Send fires a notification for the given event.
	// cfg is the JSON-decoded config map for this provider.
	Send(ctx context.Context, cfg map[string]string, e Event) error
}

// Registry maps provider type names to their implementations.
var Registry = map[string]Provider{
	"webhook":  &WebhookProvider{},
	"telegram": &TelegramProvider{},
	"email":    &EmailProvider{},
}

// SendResult holds the outcome of a single provider send attempt.
type SendResult struct {
	NotifConfig NotifConfig
	Err         error
}

// SendAll fires all active notifications linked to a monitor and returns one
// SendResult per entry. Errors are logged but do not abort remaining sends.
func SendAll(ctx context.Context, notifs []NotifConfig, e Event) []SendResult {
	results := make([]SendResult, 0, len(notifs))
	for _, n := range notifs {
		r := SendResult{NotifConfig: n}
		p, ok := Registry[n.Type]
		if !ok {
			r.Err = fmt.Errorf("unknown provider type %q", n.Type)
		} else {
			r.Err = p.Send(ctx, n.Config, e)
		}
		if r.Err != nil {
			log.Printf("notifier[%s]: send error for monitor %d: %v", n.Type, e.MonitorID, r.Err)
		}
		results = append(results, r)
	}
	return results
}

// NotifConfig is a decoded notification row passed to SendAll.
type NotifConfig struct {
	ID     int64
	Name   string
	Type   string
	Config map[string]string
}

// RequiredField returns an error if key is missing or empty in cfg.
func RequiredField(cfg map[string]string, key string) (string, error) {
	v, ok := cfg[key]
	if !ok || v == "" {
		return "", fmt.Errorf("notification config missing required field %q", key)
	}
	return v, nil
}
