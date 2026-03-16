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

// EvolutionProvider sends WhatsApp messages via the Evolution API (self-hosted WhatsApp bridge).
type EvolutionProvider struct{}

// Send sends a WhatsApp text message via the Evolution API.
// Required config fields: url (Evolution API server URL), api_key, instance, phone (recipient)
func (p *EvolutionProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	serverURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	apiKey, err := RequiredField(cfg, "api_key")
	if err != nil {
		return err
	}
	instance, err := RequiredField(cfg, "instance")
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
		"number": phone,
		"options": map[string]interface{}{
			"delay":    1200,
			"presence": "composing",
		},
		"textMessage": map[string]string{
			"text": message,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("evolution: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(serverURL, "/") + "/message/sendText/" + instance
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("evolution: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", apiKey)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("evolution: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("evolution: server returned %d", resp.StatusCode)
	}
	return nil
}
