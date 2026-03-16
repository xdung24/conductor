package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var pushPlusAPIURL = "https://www.pushplus.plus/send"

// PushPlusProvider sends WeChat notifications via the PushPlus service.
type PushPlusProvider struct{}

// Send posts a notification to PushPlus.
// Required config fields: token
// Optional config fields: topic (channel topic, omit for personal notification)
func (p *PushPlusProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	token, err := RequiredField(cfg, "token")
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	content := fmt.Sprintf("URL: %s<br>Latency: %d ms<br>%s", e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]string{
		"token":    token,
		"title":    title,
		"content":  content,
		"template": "html",
	}
	if topic := cfg["topic"]; topic != "" {
		payload["topic"] = topic
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pushplus: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushPlusAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pushplus: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushplus: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushplus: server returned %d", resp.StatusCode)
	}
	return nil
}
