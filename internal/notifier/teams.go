package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TeamsProvider sends notifications to a Microsoft Teams channel via Incoming Webhooks.
type TeamsProvider struct{}

// Send posts a MessageCard to the configured Microsoft Teams Webhook URL.
// Required config fields: url
func (p *TeamsProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	webhookURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}

	themeColor := "2ecc71"
	if e.Status == 0 {
		themeColor = "e74c3c"
	}

	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": themeColor,
		"summary":    fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
		"sections": []map[string]interface{}{
			{
				"activityTitle": fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
				"facts": []map[string]string{
					{"name": "URL", "value": e.MonitorURL},
					{"name": "Latency", "value": fmt.Sprintf("%d ms", e.LatencyMs)},
					{"name": "Message", "value": e.Message},
				},
				"markdown": true,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("teams: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("teams: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("teams: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("teams: server returned %d", resp.StatusCode)
	}
	return nil
}
