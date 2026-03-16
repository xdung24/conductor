package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var serverChanBaseURL = "https://sctapi.ftqq.com"

// ServerChanProvider sends WeChat notifications via the ServerChan (Server酱) service.
type ServerChanProvider struct{}

// Send posts a notification to ServerChan.
// Required config fields: send_key
func (p *ServerChanProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	sendKey, err := RequiredField(cfg, "send_key")
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	desp := fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message)

	endpoint := fmt.Sprintf("%s/%s.send", serverChanBaseURL, url.PathEscape(sendKey))

	payload := map[string]string{
		"title": title,
		"desp":  desp,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serverchan: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("serverchan: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("serverchan: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("serverchan: server returned %d", resp.StatusCode)
	}
	return nil
}
