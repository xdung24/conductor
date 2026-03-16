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

// SignalProvider sends notifications via the signal-cli REST API.
type SignalProvider struct{}

// Send sends a Signal message via the signal-cli REST API.
// Required config fields: url (signal-cli REST API URL), number (sender number), recipients (comma-separated)
func (p *SignalProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	apiURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	number, err := RequiredField(cfg, "number")
	if err != nil {
		return err
	}
	recipientsStr, err := RequiredField(cfg, "recipients")
	if err != nil {
		return err
	}

	rawRecipients := strings.Split(recipientsStr, ",")
	recipients := make([]string, 0, len(rawRecipients))
	for _, r := range rawRecipients {
		r = strings.TrimSpace(r)
		if r != "" {
			recipients = append(recipients, r)
		}
	}

	message := fmt.Sprintf("[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"message":    message,
		"number":     number,
		"recipients": recipients,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("signal: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(apiURL, "/") + "/v2/send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("signal: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("signal: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("signal: server returned %d", resp.StatusCode)
	}
	return nil
}
