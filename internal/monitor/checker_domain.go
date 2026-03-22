package monitor

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/xdung24/conductor/internal/models"
)

// DomainExpiryChecker queries WHOIS to determine a domain's registration
// expiry date. It reports DOWN when the domain is already expired or will
// expire within DomainExpiryAlertDays (default: 30 when 0).
//
// DNS resolution for the WHOIS server hostname is controlled via priority:
//  1. DoHURL set → DNS-over-HTTPS (RFC 8484 GET)
//  2. DNSServer set → custom plain-DNS resolver
//  3. Neither → system default
type DomainExpiryChecker struct{}

// Check performs the WHOIS lookup and evaluates the expiry date.
func (c *DomainExpiryChecker) Check(ctx context.Context, m *models.Monitor) Result {
	domain := extractDomain(m.URL)
	if domain == "" {
		return Result{Status: 0, Message: "domain name is required (set URL to e.g. example.com)"}
	}

	client := whois.NewClient()
	client.SetTimeout(time.Duration(m.TimeoutSeconds) * time.Second)

	if m.DoHURL != "" || m.DNSServer != "" {
		client.SetDialer(&customWhoisDialer{m: m})
	}

	start := time.Now()
	rawWhois, err := client.Whois(domain)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("WHOIS query failed: %v", err)}
	}

	parsed, err := whoisparser.Parse(rawWhois)
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("WHOIS parse failed: %v", err)}
	}

	if parsed.Domain == nil || (parsed.Domain.ExpirationDate == "" && parsed.Domain.ExpirationDateInTime == nil) {
		return Result{Status: 0, LatencyMs: latency, Message: "could not determine domain expiry date from WHOIS response"}
	}

	var expiry time.Time
	if parsed.Domain.ExpirationDateInTime != nil {
		expiry = parsed.Domain.ExpirationDateInTime.UTC()
	} else {
		expiry, err = parseFlexDate(parsed.Domain.ExpirationDate)
		if err != nil {
			return Result{
				Status:    0,
				LatencyMs: latency,
				Message:   fmt.Sprintf("could not parse expiry date %q: %v", parsed.Domain.ExpirationDate, err),
			}
		}
		expiry = expiry.UTC()
	}

	now := time.Now().UTC()
	if now.After(expiry) {
		daysExpired := int(now.Sub(expiry).Hours() / 24)
		return Result{
			Status:    0,
			LatencyMs: latency,
			Message:   fmt.Sprintf("domain expired %d days ago (%s)", daysExpired, expiry.Format("2006-01-02")),
		}
	}

	daysLeft := int(expiry.Sub(now).Hours() / 24)

	alertDays := m.DomainExpiryAlertDays
	if alertDays <= 0 {
		alertDays = 30
	}

	if daysLeft <= alertDays {
		return Result{
			Status:    0,
			LatencyMs: latency,
			Message:   fmt.Sprintf("domain expires in %d days (%s)", daysLeft, expiry.Format("2006-01-02")),
		}
	}

	return Result{
		Status:    1,
		LatencyMs: latency,
		Message:   fmt.Sprintf("domain valid, expires in %d days (%s)", daysLeft, expiry.Format("2006-01-02")),
	}
}

// extractDomain strips scheme, path, port, and whitespace from a raw URL or
// bare domain string, returning only the hostname portion.
func extractDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "://"); idx >= 0 {
		raw = raw[idx+3:]
	}
	// Strip path/query.
	if idx := strings.IndexAny(raw, "/?#"); idx >= 0 {
		raw = raw[:idx]
	}
	// Strip port.
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

// customWhoisDialer implements proxy.Dialer so it can be passed to
// whois.Client.SetDialer(). It routes DNS resolution through either DoH or a
// custom plain-DNS resolver as configured on the monitor.
type customWhoisDialer struct {
	m *models.Monitor
}

// Dial satisfies the proxy.Dialer interface.
func (d *customWhoisDialer) Dial(network, addr string) (net.Conn, error) {
	ctx := context.Background()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid whois address %q: %w", addr, err)
	}

	var ips []string
	if d.m.DoHURL != "" {
		ips, err = lookupViaDoh(ctx, d.m.DoHURL, host)
	} else {
		ips, err = resolverFor(d.m).LookupHost(ctx, host)
	}
	if err != nil {
		return nil, fmt.Errorf("DNS lookup for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses resolved for %q", host)
	}
	nd := net.Dialer{}
	return nd.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
}

// lookupViaDoh resolves a hostname using DNS-over-HTTPS (RFC 8484 GET variant).
// It queries for A records and returns the resolved IPv4 addresses.
func lookupViaDoh(ctx context.Context, dohURL, name string) ([]string, error) {
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	dnsName, err := dnsmessage.NewName(name)
	if err != nil {
		return nil, fmt.Errorf("build DNS name for %q: %w", name, err)
	}

	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               1,
			RecursionDesired: true,
		},
		Questions: []dnsmessage.Question{{
			Name:  dnsName,
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}},
	}

	rawQuery, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("pack DNS query: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(rawQuery)
	reqURL := dohURL + "?dns=" + encoded

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build DoH request: %w", err)
	}
	req.Header.Set("Accept", "application/dns-message")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DoH request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH server returned HTTP %d", resp.StatusCode)
	}

	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("read DoH response: %w", err)
	}

	var respMsg dnsmessage.Message
	if err := respMsg.Unpack(rawResp); err != nil {
		return nil, fmt.Errorf("unpack DoH response: %w", err)
	}

	var ips []string
	for _, ans := range respMsg.Answers {
		if body, ok := ans.Body.(*dnsmessage.AResource); ok {
			ips = append(ips, net.IP(body.A[:]).String())
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no A records for %q via DoH", strings.TrimSuffix(name, "."))
	}
	return ips, nil
}

// parseFlexDate tries several common date formats to parse a WHOIS expiry
// date string. The whoisparser library normally normalises dates, but this
// fallback handles cases where normalisation fails.
func parseFlexDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"January 2 2006",
		"02/01/2006",
		"01/02/2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format %q", s)
}
