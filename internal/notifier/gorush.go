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

// GorushProvider sends push notifications via a Gorush push server.
type GorushProvider struct{}

// Send posts a push notification to the configured Gorush server.
// Required config fields: server_url, tokens (comma-separated device tokens)
// Optional config fields: platform (default "2" = Android; "1" = iOS)
func (p *GorushProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	serverURL, err := RequiredField(cfg, "server_url")
	if err != nil {
		return err
	}
	tokensStr, err := RequiredField(cfg, "tokens")
	if err != nil {
		return err
	}

	platform := 2
	if v := cfg["platform"]; v == "1" {
		platform = 1
	}

	rawTokens := strings.Split(tokensStr, ",")
	tokens := make([]string, 0, len(rawTokens))
	for _, t := range rawTokens {
		t = strings.TrimSpace(t)
		if t != "" {
			tokens = append(tokens, t)
		}
	}

	title := fmt.Sprintf("%s is %s", e.MonitorName, e.StatusText())
	message := fmt.Sprintf("URL: %s\nLatency: %d ms\n%s", e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"notifications": []map[string]interface{}{
			{
				"tokens":   tokens,
				"platform": platform,
				"title":    title,
				"message":  message,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gorush: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(serverURL, "/") + "/api/push"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gorush: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gorush: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gorush: server returned %d", resp.StatusCode)
	}
	return nil
}
