package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xdung24/service-monitor/internal/models"
)

// RabbitMQChecker connects to the RabbitMQ management HTTP API and calls the
// node health endpoint.  The monitor URL must point to the management API root,
// e.g. http://user:pass@rabbitmq.example.com:15672.
type RabbitMQChecker struct{}

// Check performs a RabbitMQ health check via the management plugin.
//
// It calls GET {url}/api/healthchecks/node and expects {"status":"ok"}.
func (c *RabbitMQChecker) Check(ctx context.Context, m *models.Monitor) Result {
	base := m.URL
	// Strip trailing slash to avoid double-slash in the path.
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	endpoint := base + "/api/healthchecks/node"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Status: 0, Message: "build request: " + err.Error()}
	}

	if m.HTTPUsername != "" || m.HTTPPassword != "" {
		req.SetBasicAuth(m.HTTPUsername, m.HTTPPassword)
	}

	transport := &http.Transport{}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: 0, Message: err.Error()}
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return Result{Status: 0, Message: "read body: " + err.Error()}
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return Result{Status: 0, Message: "authentication required — check username/password"}
	}
	if resp.StatusCode != http.StatusOK {
		return Result{Status: 0, Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))}
	}

	var payload struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Result{Status: 0, Message: "malformed JSON: " + err.Error()}
	}

	if payload.Status != "ok" {
		reason := payload.Reason
		if reason == "" {
			reason = string(body)
		}
		return Result{Status: 0, Message: "RabbitMQ unhealthy: " + reason}
	}
	return Result{Status: 1, Message: "RabbitMQ node is healthy"}
}
