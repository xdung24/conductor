package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/xdung24/service-monitor/internal/models"
)

// ---------------------------------------------------------------------------
// WebSocket Upgrade checker
// ---------------------------------------------------------------------------

// WebSocketChecker verifies that a WebSocket endpoint accepts an upgrade
// request (HTTP 101 Switching Protocols). The monitor URL must use the
// ws:// or wss:// scheme.
type WebSocketChecker struct{}

// Check dials the WebSocket endpoint and immediately closes the connection.
// Returns UP if the upgrade handshake succeeds within the timeout period.
func (c *WebSocketChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	url := m.URL
	// Normalise http(s) schemes to ws(s) so users can enter either form.
	if strings.HasPrefix(url, "http://") {
		url = "ws://" + url[7:]
	} else if strings.HasPrefix(url, "https://") {
		url = "wss://" + url[8:]
	}

	opts := &websocket.DialOptions{
		HTTPClient: &http.Client{
			Timeout: time.Duration(m.TimeoutSeconds) * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: m.HTTPIgnoreTLS, // #nosec G402 -- user opt-in
				},
			},
		},
	}

	conn, _, err := websocket.Dial(ctx, url, opts)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("dial: %v", err)}
	}
	conn.CloseNow() //nolint:errcheck

	return Result{
		Status:    1,
		LatencyMs: latency,
		Message:   "WebSocket upgrade OK",
	}
}
