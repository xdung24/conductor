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

// HomeAssistantProvider sends notifications via a Home Assistant notify service.
type HomeAssistantProvider struct{}

// Send posts a notification to the configured Home Assistant instance.
// Required config fields: url (HA base URL), token (Long-Lived Access Token), notification_id (notify service name)
func (p *HomeAssistantProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	haURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	token, err := RequiredField(cfg, "token")
	if err != nil {
		return err
	}
	notifID, err := RequiredField(cfg, "notification_id")
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	message := fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"title":   title,
		"message": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("homeassistant: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(haURL, "/") + "/api/services/notify/" + notifID
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("homeassistant: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("homeassistant: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("homeassistant: server returned %d", resp.StatusCode)
	}
	return nil
}
