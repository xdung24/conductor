package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OneBotProvider sends QQ messages via the OneBot protocol REST API.
type OneBotProvider struct{}

// Send posts a message to the configured OneBot-compatible server.
// Required config fields: url (e.g. http://localhost:5700), to (QQ number)
// Optional config fields: type ("private" or "group", default "private")
func (p *OneBotProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	serverURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	to, err := RequiredField(cfg, "to")
	if err != nil {
		return err
	}

	msgType := cfg["type"]
	if msgType != "group" {
		msgType = "private"
	}

	message := fmt.Sprintf("[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	var payload map[string]interface{}
	if msgType == "group" {
		payload = map[string]interface{}{
			"group_id": to,
			"message":  message,
		}
	} else {
		payload = map[string]interface{}{
			"user_id": to,
			"message": message,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("onebot: marshal payload: %w", err)
	}

	endpoint := serverURL
	if msgType == "group" {
		endpoint += "/send_group_msg"
	} else {
		endpoint += "/send_private_msg"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("onebot: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	if token := cfg["token"]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("onebot: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("onebot: server returned %d", resp.StatusCode)
	}
	return nil
}
