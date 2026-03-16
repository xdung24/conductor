package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GoogleChatProvider sends notifications to a Google Chat space via Incoming Webhooks.
type GoogleChatProvider struct{}

// Send posts a text card to the configured Google Chat Webhook URL.
// Required config fields: url
func (p *GoogleChatProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	webhookURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}

	text := fmt.Sprintf("*[%s]* %s is *%s*\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"text": text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("googlechat: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("googlechat: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("googlechat: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("googlechat: server returned %d", resp.StatusCode)
	}
	return nil
}
