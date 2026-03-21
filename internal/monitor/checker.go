package monitor

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xdung24/conductor/internal/models"
)

// Result holds the outcome of a single check.
type Result struct {
	Status      int
	LatencyMs   int
	Message     string
	BodyExcerpt string // first N chars of HTTP response body; non-empty only when NotifyBodyChars > 0
}

// Checker is something that can check a monitor.
type Checker interface {
	Check(ctx context.Context, m *models.Monitor) Result
}

// DockerHostLookup is an optional callback that resolves a docker_host ID to
// its (socketPath, httpURL) connection details. The db parameter is the
// per-user database that stores the docker_hosts table.
var DockerHostLookup func(db *sql.DB, id int64) (socketPath, httpURL string)

// Run performs the appropriate check for a monitor (with retry logic).
// db is the per-user database used for per-user resource lookups (e.g. docker hosts).
func Run(ctx context.Context, db *sql.DB, m *models.Monitor) Result {
	checker := checkerFor(db, m)
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

func checkerFor(db *sql.DB, m *models.Monitor) Checker {
	switch m.Type {
	case models.MonitorTypeDNS:
		return &DNSChecker{}
	case models.MonitorTypeTCP:
		return &TCPChecker{}
	case models.MonitorTypePing:
		return &PingChecker{}
	case models.MonitorTypeSMTP:
		return &SMTPChecker{}
	case models.MonitorTypeMySQL:
		return &MySQLChecker{}
	case models.MonitorTypePostgres:
		return &PostgresChecker{}
	case models.MonitorTypeRedis:
		return &RedisChecker{}
	case models.MonitorTypeMongoDB:
		return &MongoDBChecker{}
	case models.MonitorTypeMSSQL:
		return &MSSQLChecker{}
	case models.MonitorTypeWebSocket:
		return &WebSocketChecker{}
	case models.MonitorTypeMQTT:
		return &MQTTChecker{}
	case models.MonitorTypeGRPC:
		return &GRPCChecker{}
	case models.MonitorTypeDocker:
		dc := &DockerChecker{}
		if DockerHostLookup != nil && db != nil && m.DockerHostID > 0 {
			dc.HostSocketPath, dc.HostHTTPURL = DockerHostLookup(db, m.DockerHostID)
		}
		return dc
	case models.MonitorTypeRabbitMQ:
		return &RabbitMQChecker{}
	case models.MonitorTypeSNMP:
		return &SNMPChecker{}
	case models.MonitorTypeSystemType:
		return &SystemServiceChecker{}
	case models.MonitorTypeTailscale:
		return &TailscaleChecker{}
	case models.MonitorTypeGlobalping:
		return &GlobalpingChecker{}
	case models.MonitorTypeGroup:
		return &GroupChecker{}
	case models.MonitorTypeManual:
		return &ManualChecker{}
	case models.MonitorTypeSIPOptions:
		return &SIPOptionsChecker{}
	case models.MonitorTypeKafka:
		return &KafkaChecker{}
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

// Check performs an HTTP/HTTPS request and records status + latency.
// Supports custom method, TLS skip, auth, accepted status codes, and keyword matching.
func (c *HTTPChecker) Check(ctx context.Context, m *models.Monitor) Result {
	d := dialerFor(m)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: m.HTTPIgnoreTLS}, // #nosec G402 -- user opt-in
		DialContext:     d.DialContext,
	}

	maxRedirects := m.HTTPMaxRedirects
	if maxRedirects < 0 {
		maxRedirects = 10
	}
	client := &http.Client{
		Timeout:   time.Duration(m.TimeoutSeconds) * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if maxRedirects == 0 {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			return nil
		},
	}

	method := m.HTTPMethod
	if method == "" {
		method = http.MethodGet
	}

	start := time.Now()
	var bodyReader io.Reader
	if m.HTTPRequestBody != "" {
		bodyReader = strings.NewReader(m.HTTPRequestBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.URL, bodyReader)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("invalid request: %v", err)}
	}
	req.Header.Set("User-Agent", "conductor/1.0")

	// Apply custom request headers (Key: Value, one per line).
	for _, line := range strings.Split(m.HTTPRequestHeaders, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			if k != "" {
				req.Header.Set(k, v)
			}
		}
	}

	// Bearer token takes priority over HTTP basic auth.
	if m.HTTPBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.HTTPBearerToken)
	} else if m.HTTPUsername != "" || m.HTTPPassword != "" {
		req.SetBasicAuth(m.HTTPUsername, m.HTTPPassword)
	}

	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: err.Error()}
	}
	defer resp.Body.Close()

	statusMsg := fmt.Sprintf("%d %s", resp.StatusCode, resp.Status)
	if !isStatusAccepted(resp.StatusCode, m.HTTPAcceptedStatuses) {
		return Result{Status: 0, LatencyMs: latency, Message: statusMsg}
	}

	// Header assertion (reads resp.Header only — no body needed).
	if msg := checkHeaderConstraint(resp, m); msg != "" {
		return Result{Status: 0, LatencyMs: latency, Message: msg}
	}

	// Body-type assertion (validates Content-Type header — no body read needed).
	if msg := checkBodyType(resp, m.HTTPBodyType); msg != "" {
		return Result{Status: 0, LatencyMs: latency, Message: msg}
	}

	// Read response body once when any body-based check is configured.
	var body []byte
	if m.HTTPKeyword != "" || m.HTTPJsonPath != "" || m.HTTPXPath != "" || m.NotifyBodyChars > 0 {
		body, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
		if err != nil {
			return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("read body: %v", err)}
		}
	}

	// Keyword check.
	if m.HTTPKeyword != "" {
		found := strings.Contains(string(body), m.HTTPKeyword)
		if m.HTTPKeywordInvert {
			found = !found
		}
		if !found {
			return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("keyword %q not found in response", m.HTTPKeyword)}
		}
	}

	// JSONPath assertion.
	if msg := checkJsonPath(body, m.HTTPJsonPath, m.HTTPJsonExpected); msg != "" {
		return Result{Status: 0, LatencyMs: latency, Message: msg}
	}

	// XPath assertion.
	if msg := checkXPath(body, m.HTTPXPath, m.HTTPXPathExpected); msg != "" {
		return Result{Status: 0, LatencyMs: latency, Message: msg}
	}

	r := Result{Status: 1, LatencyMs: latency, Message: statusMsg}
	if m.NotifyBodyChars > 0 && len(body) > 0 {
		excerpt := strings.TrimSpace(string(body))
		if len(excerpt) > m.NotifyBodyChars {
			excerpt = excerpt[:m.NotifyBodyChars]
		}
		r.BodyExcerpt = excerpt
	}

	// TLS certificate expiry alert: check the leaf cert's NotAfter.
	// Only fires when the request actually used TLS (resp.TLS != nil) AND the
	// user has set an alert threshold. Does not conflict with HTTPIgnoreTLS —
	// the check applies whether or not cert errors are skipped.
	if m.CertExpiryAlertDays > 0 && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		leaf := resp.TLS.PeerCertificates[0]
		daysLeft := int(time.Until(leaf.NotAfter).Hours() / 24)
		if daysLeft < m.CertExpiryAlertDays {
			if daysLeft < 0 {
				return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("TLS certificate expired %d day(s) ago", -daysLeft)}
			}
			return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("TLS certificate expires in %d day(s) (threshold: %d)", daysLeft, m.CertExpiryAlertDays)}
		}
	}

	return r
}

// isStatusAccepted reports whether code is in the accepted set.
// If accepted is empty, any 2xx or 3xx status is accepted.
// Otherwise accepted is a comma-separated list of exact integer status codes.
func isStatusAccepted(code int, accepted string) bool {
	if accepted == "" {
		return code >= 200 && code < 400
	}
	for _, part := range strings.Split(accepted, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n == code {
			return true
		}
	}
	return false
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

// ---------------------------------------------------------------------------
// DNS checker
// ---------------------------------------------------------------------------

// DNSChecker resolves a DNS record for the configured domain and optionally
// validates that an answer contains the expected value.
type DNSChecker struct{}

// Check performs a DNS lookup and records status + latency.
func (c *DNSChecker) Check(ctx context.Context, m *models.Monitor) Result {
	resolver := resolverFor(m)
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	recordType := strings.ToUpper(m.DNSRecordType)
	if recordType == "" {
		recordType = "A"
	}

	start := time.Now()
	var answers []string
	var lookupErr error

	switch recordType {
	case "A":
		ips, e := resolver.LookupIPAddr(ctx, m.URL)
		for _, ip := range ips {
			if ip.IP.To4() != nil {
				answers = append(answers, ip.IP.String())
			}
		}
		lookupErr = e
	case "AAAA":
		ips, e := resolver.LookupIPAddr(ctx, m.URL)
		for _, ip := range ips {
			if ip.IP.To4() == nil && ip.IP.To16() != nil {
				answers = append(answers, ip.IP.String())
			}
		}
		lookupErr = e
	case "CNAME":
		cname, e := resolver.LookupCNAME(ctx, m.URL)
		if e == nil {
			answers = []string{strings.TrimSuffix(cname, ".")}
		}
		lookupErr = e
	case "MX":
		mxs, e := resolver.LookupMX(ctx, m.URL)
		for _, mx := range mxs {
			answers = append(answers, fmt.Sprintf("%s (pri %d)", strings.TrimSuffix(mx.Host, "."), mx.Pref))
		}
		lookupErr = e
	case "NS":
		nss, e := resolver.LookupNS(ctx, m.URL)
		for _, ns := range nss {
			answers = append(answers, strings.TrimSuffix(ns.Host, "."))
		}
		lookupErr = e
	case "TXT":
		txts, e := resolver.LookupTXT(ctx, m.URL)
		answers = txts
		lookupErr = e
	case "PTR":
		ptrs, e := resolver.LookupAddr(ctx, m.URL)
		for _, p := range ptrs {
			answers = append(answers, strings.TrimSuffix(p, "."))
		}
		lookupErr = e
	default:
		return Result{Status: 0, Message: fmt.Sprintf("unsupported record type: %s", recordType)}
	}

	latency := int(time.Since(start).Milliseconds())
	if lookupErr != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("DNS %s lookup failed: %v", recordType, lookupErr)}
	}
	if len(answers) == 0 {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("no %s records for %s", recordType, m.URL)}
	}

	msg := fmt.Sprintf("%s %s → %s", m.URL, recordType, strings.Join(answers, ", "))
	if m.DNSExpected != "" {
		for _, a := range answers {
			if strings.Contains(a, m.DNSExpected) {
				return Result{Status: 1, LatencyMs: latency, Message: msg}
			}
		}
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("expected %q not in answers: %s", m.DNSExpected, msg)}
	}
	return Result{Status: 1, LatencyMs: latency, Message: msg}
}
