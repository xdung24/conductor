package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var whapiAPIURL = "https://gate.whapi.cloud/messages/text"

// WhapiProvider sends WhatsApp messages via the Whapi.cloud API.
type WhapiProvider struct{}

// Send sends a WhatsApp text message via Whapi.
// Required config fields: token, phone (recipient, international format)
func (p *WhapiProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	token, err := RequiredField(cfg, "token")
	if err != nil {
		return err
	}
	phone, err := RequiredField(cfg, "phone")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"to":   phone,
		"body": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("whapi: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, whapiAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("whapi: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("whapi: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whapi: server returned %d", resp.StatusCode)
	}
	return nil
}
