package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MattermostProvider sends notifications to a Mattermost channel via Incoming Webhooks.
type MattermostProvider struct{}

// Send posts a message to the configured Mattermost Incoming Webhook URL.
// Required config fields: url
func (p *MattermostProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
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
				"fallback": fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
				"color":    color,
				"title":    fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
				"text":     fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mattermost: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mattermost: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("mattermost: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mattermost: server returned %d", resp.StatusCode)
	}
	return nil
}
