package notifier

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var lineNotifyAPIURL = "https://notify-api.line.me/api/notify"

// LINEProvider sends notifications via LINE Notify.
type LINEProvider struct{}

// Send posts a line notification via LINE Notify API.
// Required config fields: token
func (p *LINEProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	token, err := RequiredField(cfg, "token")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("\n[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	form := url.Values{}
	form.Set("message", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		lineNotifyAPIURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("line: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("line: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("line: server returned %d", resp.StatusCode)
	}
	return nil
}
