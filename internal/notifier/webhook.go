package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookProvider sends a JSON POST to an arbitrary URL.
type WebhookProvider struct{}

// Send posts a JSON payload to the configured webhook URL.
// Required config fields: url
// Optional config fields: secret (added as X-Webhook-Secret header)
func (p *WebhookProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	url, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"monitor_id":   e.MonitorID,
		"monitor_name": e.MonitorName,
		"monitor_url":  e.MonitorURL,
		"status":       e.StatusText(),
		"latency_ms":   e.LatencyMs,
		"message":      e.Message,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")
	if secret := cfg["secret"]; secret != "" {
		req.Header.Set("X-Webhook-Secret", secret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: server returned %d", resp.StatusCode)
	}
	return nil
}
