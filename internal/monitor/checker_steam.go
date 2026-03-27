package monitor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/xdung24/conductor/internal/models"
)

// SteamChecker performs a Source Engine A2S_INFO UDP query.
type SteamChecker struct{}

// GameDigChecker supports a minimal subset of game server protocols.
// Supported values in m.GameDigGame: a2s (default), quake3.
type GameDigChecker struct{}

// BrowserChecker performs a Chromium-based check using chromedp.
// When RemoteEndpoint is set, it connects to a remote DevTools endpoint.
type BrowserChecker struct {
	RemoteEndpoint string
}

// Check performs a Steam A2S_INFO query.
func (c *SteamChecker) Check(ctx context.Context, m *models.Monitor) Result {
	host, port, err := hostPort(strings.TrimSpace(m.URL), "27015")
	if err != nil {
		return Result{Status: 0, Message: "invalid host: " + err.Error()}
	}
	name, latency, err := queryA2SInfo(ctx, net.JoinHostPort(host, port))
	if err != nil {
		return Result{Status: 0, Message: err.Error()}
	}
	if name == "" {
		name = "unknown"
	}
	return Result{Status: 1, LatencyMs: latency, Message: "Steam server OK: " + name}
}

// Check performs a GameDig-style UDP query using a small protocol subset.
func (c *GameDigChecker) Check(ctx context.Context, m *models.Monitor) Result {
	game := strings.ToLower(strings.TrimSpace(m.GameDigGame))
	if game == "" {
		game = "a2s"
	}

	switch game {
	case "a2s", "steam":
		host, port, err := hostPort(strings.TrimSpace(m.URL), "27015")
		if err != nil {
			return Result{Status: 0, Message: "invalid host: " + err.Error()}
		}
		name, latency, err := queryA2SInfo(ctx, net.JoinHostPort(host, port))
		if err != nil {
			return Result{Status: 0, Message: err.Error()}
		}
		if name == "" {
			name = "unknown"
		}
		return Result{Status: 1, LatencyMs: latency, Message: "GameDig(A2S) OK: " + name}
	case "quake3", "quake":
		host, port, err := hostPort(strings.TrimSpace(m.URL), "27960")
		if err != nil {
			return Result{Status: 0, Message: "invalid host: " + err.Error()}
		}
		msg, latency, err := queryQuake3Status(ctx, net.JoinHostPort(host, port))
		if err != nil {
			return Result{Status: 0, Message: err.Error()}
		}
		return Result{Status: 1, LatencyMs: latency, Message: msg}
	default:
		return Result{Status: 0, Message: "unsupported gamedig_game: " + game}
	}
}

// Check runs a real browser monitor via local or remote Chromium.
func (c *BrowserChecker) Check(ctx context.Context, m *models.Monitor) Result {
	if strings.TrimSpace(m.URL) == "" {
		return Result{Status: 0, Message: "url is required"}
	}
	u, err := url.Parse(m.URL)
	if err != nil {
		return Result{Status: 0, Message: "invalid url: " + err.Error()}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Result{Status: 0, Message: "invalid url protocol, only http and https are allowed"}
	}

	start := time.Now()
	var allocCtx context.Context
	var cancelAlloc context.CancelFunc

	if strings.TrimSpace(c.RemoteEndpoint) != "" {
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(ctx, c.RemoteEndpoint)
	} else {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.NoDefaultBrowserCheck,
			chromedp.NoFirstRun,
		)
		allocCtx, cancelAlloc = chromedp.NewExecAllocator(ctx, opts...)
	}
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	actions := []chromedp.Action{
		chromedp.Navigate(m.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	}

	var html string
	if m.HTTPKeyword != "" {
		actions = append(actions, chromedp.InnerHTML("html", &html, chromedp.ByQuery))
	}

	if err := chromedp.Run(taskCtx, actions...); err != nil {
		return Result{Status: 0, LatencyMs: int(time.Since(start).Milliseconds()), Message: "browser check failed: " + err.Error()}
	}

	if m.HTTPKeyword != "" {
		found := strings.Contains(html, m.HTTPKeyword)
		if m.HTTPKeywordInvert {
			found = !found
		}
		if !found {
			return Result{Status: 0, LatencyMs: int(time.Since(start).Milliseconds()), Message: fmt.Sprintf("keyword %q not found in rendered page", m.HTTPKeyword)}
		}
	}

	latency := int(time.Since(start).Milliseconds())
	if strings.TrimSpace(c.RemoteEndpoint) != "" {
		return Result{Status: 1, LatencyMs: latency, Message: "browser OK (remote)"}
	}
	return Result{Status: 1, LatencyMs: latency, Message: "browser OK"}
}

func queryA2SInfo(ctx context.Context, addr string) (name string, latencyMs int, err error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", addr)
	if err != nil {
		return "", 0, fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	if d, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(d); err != nil {
			return "", 0, fmt.Errorf("set deadline: %w", err)
		}
	}

	query := []byte("\xff\xff\xff\xff\x54Source Engine Query\x00")
	start := time.Now()
	if _, err := conn.Write(query); err != nil {
		return "", 0, fmt.Errorf("send a2s query: %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return "", 0, fmt.Errorf("read a2s response: %w", err)
	}
	latencyMs = int(time.Since(start).Milliseconds())
	packet := buf[:n]

	if len(packet) < 5 || !bytes.Equal(packet[:4], []byte{0xff, 0xff, 0xff, 0xff}) {
		return "", latencyMs, fmt.Errorf("invalid a2s packet")
	}

	kind := packet[4]
	if kind == 0x41 {
		if len(packet) < 9 {
			return "", latencyMs, fmt.Errorf("invalid a2s challenge packet")
		}
		challenge := packet[5:9]
		queryWithChallenge := append(append([]byte{}, query...), challenge...)
		start = time.Now()
		if _, err := conn.Write(queryWithChallenge); err != nil {
			return "", 0, fmt.Errorf("send a2s challenge query: %w", err)
		}
		n, err = conn.Read(buf)
		if err != nil {
			return "", 0, fmt.Errorf("read a2s challenge response: %w", err)
		}
		latencyMs = int(time.Since(start).Milliseconds())
		packet = buf[:n]
		if len(packet) < 5 || !bytes.Equal(packet[:4], []byte{0xff, 0xff, 0xff, 0xff}) {
			return "", latencyMs, fmt.Errorf("invalid challenged a2s packet")
		}
		kind = packet[4]
	}

	if kind != 0x49 {
		return "", latencyMs, fmt.Errorf("unexpected a2s response type: 0x%x", kind)
	}

	payload := packet[5:]
	if len(payload) < 2 {
		return "", latencyMs, fmt.Errorf("truncated a2s payload")
	}

	// protocol byte then null-terminated server name.
	_, rest := payload[0], payload[1:]
	serverName, _, err := readNulString(rest)
	if err != nil {
		return "", latencyMs, fmt.Errorf("parse server name: %w", err)
	}
	return serverName, latencyMs, nil
}

func queryQuake3Status(ctx context.Context, addr string) (msg string, latencyMs int, err error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", addr)
	if err != nil {
		return "", 0, fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	if d, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(d); err != nil {
			return "", 0, fmt.Errorf("set deadline: %w", err)
		}
	}

	query := []byte("\xff\xff\xff\xffgetstatus\n")
	start := time.Now()
	if _, err := conn.Write(query); err != nil {
		return "", 0, fmt.Errorf("send quake3 status query: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", 0, fmt.Errorf("read quake3 status response: %w", err)
	}
	latencyMs = int(time.Since(start).Milliseconds())

	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "\xff\xff\xff\xffstatusResponse\n") {
		return "", latencyMs, fmt.Errorf("unexpected quake3 response")
	}

	lines := strings.Split(resp, "\n")
	serverInfo := "statusResponse"
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		serverInfo = strings.TrimSpace(lines[1])
	}
	return "GameDig(Quake3) OK: " + serverInfo, latencyMs, nil
}

func readNulString(b []byte) (s string, rest []byte, err error) {
	i := bytes.IndexByte(b, 0x00)
	if i < 0 {
		return "", nil, fmt.Errorf("missing nul terminator")
	}
	return string(b[:i]), b[i+1:], nil
}
