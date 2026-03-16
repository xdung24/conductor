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

// MatrixProvider sends notifications via the Matrix protocol to a room.
type MatrixProvider struct{}

// Send posts a text message to the configured Matrix room.
// Required config fields: homeserver_url, access_token, room_id
func (p *MatrixProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	homeserverURL, err := RequiredField(cfg, "homeserver_url")
	if err != nil {
		return err
	}
	accessToken, err := RequiredField(cfg, "access_token")
	if err != nil {
		return err
	}
	roomID, err := RequiredField(cfg, "room_id")
	if err != nil {
		return err
	}

	msgBody := fmt.Sprintf("[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	txnID := fmt.Sprintf("sm-%d-%d", e.MonitorID, time.Now().UnixNano())

	payload := map[string]interface{}{
		"msgtype": "m.text",
		"body":    msgBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("matrix: marshal payload: %w", err)
	}

	// PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}
	roomIDEncoded := strings.ReplaceAll(roomID, "!", "%21")
	roomIDEncoded = strings.ReplaceAll(roomIDEncoded, ":", "%3A")
	endpoint := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		strings.TrimRight(homeserverURL, "/"), roomIDEncoded, txnID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("matrix: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("matrix: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("matrix: server returned %d", resp.StatusCode)
	}
	return nil
}
