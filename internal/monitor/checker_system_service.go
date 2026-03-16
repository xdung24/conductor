package monitor

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/xdung24/service-monitor/internal/models"
)

// SystemServiceChecker checks whether a named system service (systemd on Linux,
// Service Control Manager on Windows) is running.
//
// The service name is stored in m.ServiceName.
type SystemServiceChecker struct{}

// Check verifies that the named service is active/running.
func (c *SystemServiceChecker) Check(ctx context.Context, m *models.Monitor) Result {
	name := m.ServiceName
	if name == "" {
		// Fall back to URL field if service_name is not set.
		name = m.URL
	}
	if name == "" {
		return Result{Status: 0, Message: "service_name is required"}
	}

	switch runtime.GOOS {
	case "linux", "freebsd":
		return checkSystemd(ctx, name)
	case "darwin":
		return checkLaunchctl(ctx, name)
	case "windows":
		return checkWindowsSCM(ctx, name)
	default:
		return Result{Status: 0, Message: "system service checks not supported on " + runtime.GOOS}
	}
}

// checkSystemd uses `systemctl is-active {name}` (Linux).
func checkSystemd(ctx context.Context, service string) Result {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", service) //nolint:gosec
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		return Result{Status: 1, Message: service + " is active"}
	}
	// Exit code 3 means "inactive" (valid, not an execution error).
	out := strings.TrimSpace(errBuf.String())
	if out == "" {
		return Result{Status: 0, Message: service + " is not active"}
	}
	return Result{Status: 0, Message: service + ": " + out}
}

// checkLaunchctl uses `launchctl list {name}` (macOS).
func checkLaunchctl(ctx context.Context, service string) Result {
	cmd := exec.CommandContext(ctx, "launchctl", "list", service) //nolint:gosec
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = service + " not found or not running"
		}
		return Result{Status: 0, Message: msg}
	}
	// launchctl list outputs JSON with "PID" field when running.
	output := outBuf.String()
	if strings.Contains(output, `"PID"`) && !strings.Contains(output, `"PID" = 0`) {
		return Result{Status: 1, Message: service + " is running"}
	}
	return Result{Status: 0, Message: service + " is not running (no PID)"}
}

// checkWindowsSCM uses `sc.exe query {name}` (Windows).
func checkWindowsSCM(ctx context.Context, service string) Result {
	cmd := exec.CommandContext(ctx, "sc.exe", "query", service) //nolint:gosec
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = "sc.exe failed: " + err.Error()
		}
		return Result{Status: 0, Message: msg}
	}

	output := outBuf.String()
	if strings.Contains(output, "RUNNING") {
		return Result{Status: 1, Message: service + " is running"}
	}
	if strings.Contains(output, "STOPPED") {
		return Result{Status: 0, Message: service + " is stopped"}
	}
	// Return the raw state line for diagnostics.
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "STATE") {
			return Result{Status: 0, Message: strings.TrimSpace(line)}
		}
	}
	return Result{Status: 0, Message: "service state unknown: " + strings.TrimSpace(output)}
}
