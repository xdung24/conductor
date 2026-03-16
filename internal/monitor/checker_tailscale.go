package monitor

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/xdung24/service-monitor/internal/models"
)

// TailscaleChecker runs `tailscale ping --c 1 <hostname>` and interprets the
// result.  The target hostname / IP is taken from m.URL.
//
// Requires the `tailscale` CLI to be installed and authenticated on the host.
type TailscaleChecker struct{}

// Check performs a single Tailscale ping to the monitor's URL/hostname.
func (c *TailscaleChecker) Check(ctx context.Context, m *models.Monitor) Result {
	host := m.URL
	if host == "" {
		return Result{Status: 0, Message: "url (hostname) is required"}
	}

	cmd := exec.CommandContext(ctx, "tailscale", "ping", "--c", "1", host) //nolint:gosec
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout := strings.TrimSpace(outBuf.String())
	stderr := strings.TrimSpace(errBuf.String())

	if err != nil {
		msg := stderr
		if msg == "" {
			msg = stdout
		}
		if msg == "" {
			msg = err.Error()
		}
		return Result{Status: 0, Message: "tailscale ping failed: " + msg}
	}

	// Successful ping output contains "pong from" or "via DERP".
	if strings.Contains(stdout, "pong from") || strings.Contains(stdout, "via DERP") {
		return Result{Status: 1, Message: stdout}
	}
	// Any other non-error exit is still UP.
	if stdout != "" {
		return Result{Status: 1, Message: stdout}
	}
	return Result{Status: 1, Message: "tailscale ping OK"}
}
