package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/xdung24/conductor/internal/models"
)

// SSLCertChecker dials a TLS endpoint and inspects the leaf certificate's
// validity window independently of any HTTP response content.
//
// URL format: "hostname" or "hostname:port" — port 443 is used when omitted.
// Supports a scheme prefix (e.g. "https://hostname") which is stripped before dialing.
// Uses the monitor's DNSServer field (if set) for custom DNS resolution.
type SSLCertChecker struct{}

// Check dials the TLS endpoint, reads the leaf certificate, and returns:
//   - DOWN if the certificate is already expired
//   - DOWN if the certificate expires within CertExpiryAlertDays (default: 30 when 0)
//   - UP with days-remaining in the message otherwise
func (c *SSLCertChecker) Check(ctx context.Context, m *models.Monitor) Result {
	host, addr := sslHostAddr(m.URL)

	start := time.Now()
	dialer := dialerFor(m)
	conn, err := tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{ServerName: host})
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("TLS dial failed: %v", err)}
	}
	defer conn.Close() //nolint:errcheck

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return Result{Status: 0, LatencyMs: latency, Message: "no certificates presented by server"}
	}

	leaf := certs[0]
	now := time.Now().UTC()

	if now.After(leaf.NotAfter) {
		daysExpired := int(now.Sub(leaf.NotAfter).Hours() / 24)
		return Result{
			Status:    0,
			LatencyMs: latency,
			Message: fmt.Sprintf("certificate expired %d days ago (CN=%s, expired %s)",
				daysExpired, leaf.Subject.CommonName, leaf.NotAfter.UTC().Format("2006-01-02")),
		}
	}

	daysLeft := int(leaf.NotAfter.Sub(now).Hours() / 24)

	alertDays := m.CertExpiryAlertDays
	if alertDays <= 0 {
		alertDays = 30
	}

	if daysLeft <= alertDays {
		return Result{
			Status:    0,
			LatencyMs: latency,
			Message: fmt.Sprintf("certificate expires in %d days (CN=%s, expires %s)",
				daysLeft, leaf.Subject.CommonName, leaf.NotAfter.UTC().Format("2006-01-02")),
		}
	}

	return Result{
		Status:    1,
		LatencyMs: latency,
		Message: fmt.Sprintf("certificate valid, %d days remaining (CN=%s, expires %s)",
			daysLeft, leaf.Subject.CommonName, leaf.NotAfter.UTC().Format("2006-01-02")),
	}
}

// sslHostAddr returns (serverNameForTLS, dialAddress) from a raw URL/host string.
// Accepts: "example.com", "example.com:8443", "https://example.com", "https://example.com:8443".
func sslHostAddr(raw string) (host, addr string) {
	raw = strings.TrimSpace(raw)
	// Strip scheme.
	if idx := strings.Index(raw, "://"); idx >= 0 {
		raw = raw[idx+3:]
	}
	// Strip path/query.
	if idx := strings.IndexAny(raw, "/?#"); idx >= 0 {
		raw = raw[:idx]
	}
	host = raw
	addr = raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h // has explicit port
	} else {
		addr = net.JoinHostPort(raw, "443") // default port
	}
	return
}
