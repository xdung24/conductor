package monitor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/xdung24/service-monitor/internal/models"
)

// SIPOptionsChecker sends a raw SIP OPTIONS request over UDP and checks for a
// valid SIP response.  The monitor URL must be host:port (default port 5060).
type SIPOptionsChecker struct{}

// Check sends a SIP OPTIONS ping and waits for any SIP/2.0 response.
func (c *SIPOptionsChecker) Check(ctx context.Context, m *models.Monitor) Result {
	addr := m.URL
	// Strip leading "sip:" scheme if present.
	addr = strings.TrimPrefix(addr, "sip:")

	host, port, err := hostPort(addr, "5060")
	if err != nil {
		return Result{Status: 0, Message: "invalid address: " + err.Error()}
	}
	target := net.JoinHostPort(host, port)

	conn, err := net.Dial("udp", target)
	if err != nil {
		return Result{Status: 0, Message: "dial UDP: " + err.Error()}
	}
	defer conn.Close() //nolint:errcheck

	// Apply context deadline to I/O.
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline) //nolint:errcheck
	} else {
		conn.SetDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
	}

	localAddr := conn.LocalAddr().String()
	localHost, localPort, _ := net.SplitHostPort(localAddr)

	req := buildSIPOptions(host, target, localHost, localPort)
	if _, err := fmt.Fprint(conn, req); err != nil {
		return Result{Status: 0, Message: "send: " + err.Error()}
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return Result{Status: 0, Message: "no SIP response: " + err.Error()}
	}

	response := strings.TrimSpace(string(buf[:n]))
	firstLine := strings.SplitN(response, "\r\n", 2)[0]
	if strings.HasPrefix(firstLine, "SIP/2.0") {
		return Result{Status: 1, Message: firstLine}
	}
	return Result{Status: 0, Message: "unexpected response: " + firstLine}
}

// buildSIPOptions creates a minimal SIP OPTIONS message.
func buildSIPOptions(targetHost, targetAddr, localHost, localPort string) string {
	branch := fmt.Sprintf("z9hG4bK%d", time.Now().UnixNano())
	callID := fmt.Sprintf("%d@service-monitor", time.Now().UnixNano())
	return fmt.Sprintf(
		"OPTIONS sip:%s SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP %s;branch=%s\r\n"+
			"Max-Forwards: 70\r\n"+
			"To: <sip:%s>\r\n"+
			"From: <sip:monitor@service-monitor>;tag=sm001\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: 1 OPTIONS\r\n"+
			"Contact: <sip:monitor@%s:%s>\r\n"+
			"Accept: application/sdp\r\n"+
			"Content-Length: 0\r\n\r\n",
		targetHost,
		net.JoinHostPort(localHost, localPort), branch,
		targetAddr,
		callID,
		localHost, localPort,
	)
}
