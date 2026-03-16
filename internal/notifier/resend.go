package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var resendAPIURL = "https://api.resend.com/emails"

// ResendProvider sends transactional email via the Resend API.
type ResendProvider struct{}

// Send sends an email notification via Resend.
// Required config fields: api_key, from (sender address), to (recipient address)
func (p *ResendProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	apiKey, err := RequiredField(cfg, "api_key")
	if err != nil {
		return err
	}
	from, err := RequiredField(cfg, "from")
	if err != nil {
		return err
	}
	to, err := RequiredField(cfg, "to")
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("[%s] %s is %s", e.StatusText(), e.MonitorName, e.StatusText())
	text := fmt.Sprintf("Monitor: %s\nURL: %s\nStatus: %s\nLatency: %d ms\n%s",
		e.MonitorName, e.MonitorURL, e.StatusText(), e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resend: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend: server returned %d", resp.StatusCode)
	}
	return nil
}
