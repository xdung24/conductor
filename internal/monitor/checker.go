package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/xdung24/service-monitor/internal/models"
)

// Result holds the outcome of a single check.
type Result struct {
	Status    int // 1=up, 0=down
	LatencyMs int
	Message   string
}

// Checker is something that can check a monitor.
type Checker interface {
	Check(ctx context.Context, m *models.Monitor) Result
}

// Run performs the appropriate check for a monitor (with retry logic).
func Run(ctx context.Context, m *models.Monitor) Result {
	checker := checkerFor(m)
	timeout := time.Duration(m.TimeoutSeconds) * time.Second

	var last Result
	for attempt := 0; attempt <= m.Retries; attempt++ {
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		last = checker.Check(checkCtx, m)
		cancel()
		if last.Status == 1 {
			return last
		}
	}
	return last
}

func checkerFor(m *models.Monitor) Checker {
	switch m.Type {
	case models.MonitorTypeTCP:
		return &TCPChecker{}
	case models.MonitorTypePing:
		return &PingChecker{}
	default:
		return &HTTPChecker{}
	}
}

// resolverFor returns a custom net.Resolver that uses the monitor's configured
// DNS server (host:port). If DNSServer is empty, it returns nil so callers use
// the system default.
func resolverFor(m *models.Monitor) *net.Resolver {
	if m.DNSServer == "" {
		return nil
	}
	server := m.DNSServer
	// Ensure the address includes a port; default DNS port is 53.
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", server)
		},
	}
}

// dialerFor returns a net.Dialer wired with the monitor's custom resolver (if
// any). A nil resolver means the system default is used.
func dialerFor(m *models.Monitor) net.Dialer {
	return net.Dialer{Resolver: resolverFor(m)}
}

// ---------------------------------------------------------------------------
// HTTP checker
// ---------------------------------------------------------------------------

// HTTPChecker checks an HTTP/HTTPS endpoint.
type HTTPChecker struct{}

// Check performs an HTTP GET and records status + latency.
func (c *HTTPChecker) Check(ctx context.Context, m *models.Monitor) Result {
	d := dialerFor(m)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		DialContext:     d.DialContext,
	}
	client := &http.Client{
		Timeout:   time.Duration(m.TimeoutSeconds) * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("invalid request: %v", err)}
	}
	req.Header.Set("User-Agent", "service-monitor/1.0")

	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return Result{Status: 1, LatencyMs: latency, Message: fmt.Sprintf("%d %s", resp.StatusCode, resp.Status)}
	}
	return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("%d %s", resp.StatusCode, resp.Status)}
}

// ---------------------------------------------------------------------------
// TCP checker
// ---------------------------------------------------------------------------

// TCPChecker checks that a TCP port is open.
type TCPChecker struct{}

// Check dials a TCP address and records success/failure.
func (c *TCPChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()
	d := dialerFor(m)
	conn, err := d.DialContext(ctx, "tcp", m.URL)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: err.Error()}
	}
	conn.Close()
	return Result{Status: 1, LatencyMs: latency, Message: "TCP connection successful"}
}

// ---------------------------------------------------------------------------
// Ping checker
// ---------------------------------------------------------------------------

// PingChecker checks host reachability via a TCP connect to port 80 (ICMP
// requires raw sockets and root privileges — TCP echo is a portable proxy).
type PingChecker struct{}

// Check attempts a TCP connect to port 80 as a reachability proxy for ping.
func (c *PingChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()
	d := dialerFor(m)
	host := m.URL
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "80"))
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		// Try port 443 as fallback
		conn2, err2 := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
		if err2 != nil {
			return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("unreachable: %v", err)}
		}
		conn2.Close()
		return Result{Status: 1, LatencyMs: int(time.Since(start).Milliseconds()), Message: "reachable (port 443)"}
	}
	conn.Close()
	return Result{Status: 1, LatencyMs: latency, Message: "reachable (port 80)"}
}
