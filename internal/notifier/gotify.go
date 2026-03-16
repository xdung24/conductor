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

// GotifyProvider sends push notifications via a self-hosted Gotify server.
type GotifyProvider struct{}

// Send posts a message to the configured Gotify server.
// Required config fields: url, token
// Optional config fields: priority (default 5)
func (p *GotifyProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	serverURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	token, err := RequiredField(cfg, "token")
	if err != nil {
		return err
	}

	priority := 5
	if e.Status == 0 {
		priority = 8
	}
	if v := cfg["priority"]; v != "" {
		var p int
		if _, err2 := fmt.Sscanf(v, "%d", &p); err2 == nil {
			priority = p
		}
	}

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	message := fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"title":    title,
		"message":  message,
		"priority": priority,
		"extras": map[string]interface{}{
			"client::display": map[string]string{
				"contentType": "text/plain",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gotify: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(serverURL, "/") + "/message?token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gotify: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gotify: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gotify: server returned %d", resp.StatusCode)
	}
	return nil
}
