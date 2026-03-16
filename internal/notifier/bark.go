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

// BarkProvider sends iOS push notifications via the Bark app.
type BarkProvider struct{}

// Send posts a push notification to the Bark server.
// Required config fields: device_key
// Optional config fields: server_url (default https://api.day.app)
func (p *BarkProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	deviceKey, err := RequiredField(cfg, "device_key")
	if err != nil {
		return err
	}

	serverURL := cfg["server_url"]
	if serverURL == "" {
		serverURL = "https://api.day.app"
	}
	serverURL = strings.TrimRight(serverURL, "/")

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	body := fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message)

	level := "active"
	if e.Status == 0 {
		level = "timeSensitive"
	}

	payload := map[string]interface{}{
		"title": title,
		"body":  body,
		"level": level,
		"icon":  "https://uptime.kuma.pet/icon.png",
	}

	rawBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("bark: marshal payload: %w", err)
	}

	endpoint := serverURL + "/" + deviceKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return fmt.Errorf("bark: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("bark: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bark: server returned %d", resp.StatusCode)
	}
	return nil
}
