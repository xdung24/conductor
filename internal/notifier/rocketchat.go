package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RocketChatProvider sends notifications to Rocket.Chat via Incoming Webhooks.
type RocketChatProvider struct{}

// Send posts a message to the configured Rocket.Chat Incoming Webhook URL.
// Required config fields: url
func (p *RocketChatProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	webhookURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}

	color := "#2ecc71"
	if e.Status == 0 {
		color = "#e74c3c"
	}

	payload := map[string]interface{}{
		"username": "Service Monitor",
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
				"text":  fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message),
				"ts":    time.Now().Unix(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("rocketchat: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("rocketchat: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("rocketchat: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rocketchat: server returned %d", resp.StatusCode)
	}
	return nil
}
