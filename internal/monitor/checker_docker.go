package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/xdung24/service-monitor/internal/models"
)

// ---------------------------------------------------------------------------
// Docker Container checker
// ---------------------------------------------------------------------------

// dockerContainerState mirrors the relevant fields from the Docker
// GET /containers/{id}/json response.
type dockerContainerState struct {
	Status     string `json:"Status"`
	Running    bool   `json:"Running"`
	Paused     bool   `json:"Paused"`
	Restarting bool   `json:"Restarting"`
	Health     *struct {
		Status string `json:"Status"`
	} `json:"Health"`
}

type dockerContainerInspect struct {
	State *dockerContainerState `json:"State"`
}

// DockerChecker checks whether a Docker container is running and healthy by
// querying the Docker daemon API (GET /containers/{id}/json).
//
// Monitor field usage:
//
//	DockerContainerID — container name or short ID (required)
//	DockerHostID      — ID of the docker_hosts row; 0 = use local Unix socket
//
// The docker_hosts row provides either a socket_path or an http_url for
// connecting to the Docker daemon. When both are empty, the checker falls back
// to the default Unix socket at /var/run/docker.sock.
type DockerChecker struct {
	// HostSocketPath and HostHTTPURL are populated by the scheduler before
	// calling Check, based on the docker_hosts DB row for DockerHostID.
	HostSocketPath string
	HostHTTPURL    string
}

// Check inspects the container via the Docker API and returns UP when it is
// running and (if a health check is configured) healthy.
func (c *DockerChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	if m.DockerContainerID == "" {
		return Result{Status: 0, Message: "docker_container_id is required"}
	}

	httpClient, baseURL := c.buildClient(m)

	url := fmt.Sprintf("%s/containers/%s/json", baseURL, m.DockerContainerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("build request: %v", err)}
	}

	resp, err := httpClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("api request: %v", err)}
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("read response: %v", err)}
	}

	if resp.StatusCode == http.StatusNotFound {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("container %q not found", m.DockerContainerID)}
	}
	if resp.StatusCode != http.StatusOK {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("docker api: %d %s", resp.StatusCode, resp.Status)}
	}

	var inspect dockerContainerInspect
	if err := json.Unmarshal(body, &inspect); err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("parse response: %v", err)}
	}

	if inspect.State == nil {
		return Result{Status: 0, LatencyMs: latency, Message: "container state unavailable"}
	}

	state := inspect.State

	if !state.Running {
		return Result{Status: 0, LatencyMs: latency,
			Message: fmt.Sprintf("container state is %s", state.Status)}
	}
	if state.Paused {
		return Result{Status: 0, LatencyMs: latency, Message: "container is paused"}
	}
	if state.Restarting {
		return Result{Status: 0, LatencyMs: latency, Message: "container is restarting"}
	}

	// If the container has a health check, report its status.
	if state.Health != nil && state.Health.Status != "" && state.Health.Status != "none" {
		switch state.Health.Status {
		case "healthy":
			return Result{Status: 1, LatencyMs: latency, Message: "container running and healthy"}
		case "unhealthy":
			return Result{Status: 0, LatencyMs: latency, Message: "container is unhealthy according to its healthcheck"}
		default:
			return Result{Status: 1, LatencyMs: latency,
				Message: fmt.Sprintf("container running, health: %s", state.Health.Status)}
		}
	}

	return Result{Status: 1, LatencyMs: latency, Message: "container running"}
}

// buildClient returns an http.Client configured for the Docker daemon and the
// base URL to use for API calls.
func (c *DockerChecker) buildClient(m *models.Monitor) (*http.Client, string) {
	timeout := time.Duration(m.TimeoutSeconds) * time.Second

	// Prefer socket connection if a socket path is configured.
	socketPath := c.HostSocketPath
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	if c.HostHTTPURL != "" {
		// Use TCP HTTP connection to remote Docker daemon.
		return &http.Client{Timeout: timeout}, c.HostHTTPURL
	}

	// Use Unix socket connection to local Docker daemon.
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &http.Client{Timeout: timeout, Transport: transport}, "http://localhost"
}
