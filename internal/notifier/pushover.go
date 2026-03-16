package notifier

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var pushoverAPIURL = "https://api.pushover.net/1/messages.json"

// PushoverProvider sends push notifications via the Pushover service.
type PushoverProvider struct{}

// Send posts a Pushover notification.
// Required config fields: user_key, api_token
// Optional config fields: device (specific device name)
func (p *PushoverProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	userKey, err := RequiredField(cfg, "user_key")
	if err != nil {
		return err
	}
	apiToken, err := RequiredField(cfg, "api_token")
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	message := fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message)

	priority := "0"
	if e.Status == 0 {
		priority = "1"
	}

	form := url.Values{}
	form.Set("token", apiToken)
	form.Set("user", userKey)
	form.Set("title", title)
	form.Set("message", message)
	form.Set("priority", priority)
	if device := cfg["device"]; device != "" {
		form.Set("device", device)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushoverAPIURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("pushover: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushover: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushover: server returned %d", resp.StatusCode)
	}
	return nil
}
