package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OneSenderProvider sends WhatsApp messages via the OneSender API.
type OneSenderProvider struct{}

// Send sends a WhatsApp message via OneSender.
// Required config fields: url (OneSender server URL), token, phone
func (p *OneSenderProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	serverURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
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
		"target":  phone,
		"message": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("onesender: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(serverURL, "/") + "/api/send/message/text"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("onesender: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("onesender: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("onesender: server returned %d", resp.StatusCode)
	}
	return nil
}
