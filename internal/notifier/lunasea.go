package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LunaSeaProvider sends notifications to a LunaSea self-hosted push server.
type LunaSeaProvider struct{}

// Send posts a notification to the configured LunaSea Webhook or device endpoint.
// Required config fields: url
func (p *LunaSeaProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	webhookURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"title": fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText()),
		"body":  fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("lunasea: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("lunasea: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("lunasea: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lunasea: server returned %d", resp.StatusCode)
	}
	return nil
}
