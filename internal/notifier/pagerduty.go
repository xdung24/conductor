package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var pagerdutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutyProvider sends alerts via the PagerDuty Events API v2.
type PagerDutyProvider struct{}

// Send fires a PagerDuty trigger or resolve event.
// Required config fields: routing_key
// Optional config fields: severity (critical|error|warning|info; default "critical" on DOWN, "info" on UP)
func (p *PagerDutyProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	routingKey, err := RequiredField(cfg, "routing_key")
	if err != nil {
		return err
	}

	severity := cfg["severity"]

	var eventAction string
	if e.Status == 1 {
		eventAction = "resolve"
		if severity == "" {
			severity = "info"
		}
	} else {
		eventAction = "trigger"
		if severity == "" {
			severity = "critical"
		}
	}

	payload := map[string]interface{}{
		"routing_key":  routingKey,
		"event_action": eventAction,
		"payload": map[string]interface{}{
			"summary":  fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
			"source":   e.MonitorURL,
			"severity": severity,
			"custom_details": map[string]interface{}{
				"latency_ms": e.LatencyMs,
				"message":    e.Message,
			},
		},
		"dedup_key": fmt.Sprintf("service-monitor-%d", e.MonitorID),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pagerduty: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pagerdutyEventsURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pagerduty: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pagerduty: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pagerduty: server returned %d", resp.StatusCode)
	}
	return nil
}
