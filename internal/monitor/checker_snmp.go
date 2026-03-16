package monitor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/xdung24/service-monitor/internal/models"
)

// SNMPChecker queries a single OID via SNMP and optionally asserts the returned
// value matches an expected string.
//
// Supported versions: "1", "2c" (default), "3" (no-auth/no-priv only for v3).
// The monitor URL must be the target host (host or host:port, default port 161).
type SNMPChecker struct{}

// Check performs an SNMP GET of m.SNMPOid against m.URL.
func (c *SNMPChecker) Check(ctx context.Context, m *models.Monitor) Result {
	if m.SNMPOid == "" {
		return Result{Status: 0, Message: "snmp_oid is required"}
	}

	host, port, err := hostPort(m.URL, "161")
	if err != nil {
		return Result{Status: 0, Message: "invalid host: " + err.Error()}
	}
	portUint, err2 := parsePort(port)
	if err2 != nil {
		return Result{Status: 0, Message: "invalid port: " + err2.Error()}
	}

	community := m.SNMPCommunity
	if community == "" {
		community = "public"
	}

	version := gosnmp.Version2c
	switch m.SNMPVersion {
	case "1":
		version = gosnmp.Version1
	case "3":
		version = gosnmp.Version3
	}

	deadline, ok := ctx.Deadline()
	timeout := 10 * time.Second
	if ok {
		timeout = time.Until(deadline)
		if timeout <= 0 {
			return Result{Status: 0, Message: "context deadline exceeded"}
		}
	}

	g := &gosnmp.GoSNMP{
		Target:    host,
		Port:      portUint,
		Community: community,
		Version:   version,
		Timeout:   timeout,
		Retries:   0,
	}
	if err := g.Connect(); err != nil {
		return Result{Status: 0, Message: "connect: " + err.Error()}
	}
	defer g.Conn.Close() //nolint:errcheck

	result, err := g.Get([]string{m.SNMPOid})
	if err != nil {
		return Result{Status: 0, Message: "SNMP GET: " + err.Error()}
	}

	if len(result.Variables) == 0 {
		return Result{Status: 0, Message: "no SNMP variables returned"}
	}

	v := result.Variables[0]
	if v.Type == gosnmp.NoSuchObject || v.Type == gosnmp.NoSuchInstance {
		return Result{Status: 0, Message: fmt.Sprintf("OID %s: no such object/instance", m.SNMPOid)}
	}

	val := fmt.Sprintf("%v", v.Value)
	// For OctetString the value is a []byte — convert to string.
	if bs, ok := v.Value.([]byte); ok {
		val = string(bs)
	}

	if m.SNMPExpected != "" {
		if !matchesSNMP(val, m.SNMPExpected) {
			return Result{Status: 0, Message: fmt.Sprintf("value %q does not match expected %q", val, m.SNMPExpected)}
		}
	}

	typeName := v.Type.String()
	return Result{Status: 1, Message: fmt.Sprintf("OID %s = %s (%s)", m.SNMPOid, val, typeName)}
}

// matchesSNMP returns true if the actual value equals or contains expected.
// Case-insensitive substring match (same as HTTP keyword check).
func matchesSNMP(actual, expected string) bool {
	return strings.Contains(strings.ToLower(actual), strings.ToLower(expected))
}

// hostPort splits "host:port" and fills in defaultPort if no port is specified.
func hostPort(addr, defaultPort string) (host, port string, err error) {
	if strings.ContainsRune(addr, ':') {
		host, port, err = net.SplitHostPort(addr)
		return
	}
	return addr, defaultPort, nil
}

// parsePort converts a port string to uint16.
func parsePort(s string) (uint16, error) {
	var p int
	if _, err := fmt.Sscanf(s, "%d", &p); err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("port %q out of range", s)
	}
	return uint16(p), nil
}
