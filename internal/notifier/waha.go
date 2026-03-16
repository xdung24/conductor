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

// WAHAProvider sends WhatsApp messages via the WAHA (WhatsApp HTTP API) server.
type WAHAProvider struct{}

// Send sends a WhatsApp message via a WAHA server.
// Required config fields: url (WAHA server URL), phone (recipient phone in international format)
// Optional config fields: session (WAHA session name, default "default")
func (p *WAHAProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	serverURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	phone, err := RequiredField(cfg, "phone")
	if err != nil {
		return err
	}

	session := cfg["session"]
	if session == "" {
		session = "default"
	}

	message := fmt.Sprintf("[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"session": session,
		"chatId":  phone + "@c.us",
		"text":    message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("waha: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(serverURL, "/") + "/api/sendText"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("waha: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("waha: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("waha: server returned %d", resp.StatusCode)
	}
	return nil
}
