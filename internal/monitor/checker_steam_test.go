package monitor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/xdung24/conductor/internal/models"
)

func TestSteamChecker_Check(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer pc.Close() //nolint:errcheck

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil || n == 0 {
			return
		}
		resp := append([]byte{0xff, 0xff, 0xff, 0xff, 0x49, 0x11}, []byte("Test Steam Server\x00")...)
		_, _ = pc.WriteTo(resp, addr)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m := &models.Monitor{URL: pc.LocalAddr().String()}
	res := (&SteamChecker{}).Check(ctx, m)
	if res.Status != 1 {
		t.Fatalf("status=%d msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "Test Steam Server") {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestGameDigChecker_Quake3(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer pc.Close() //nolint:errcheck

	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil || n == 0 {
			return
		}
		if !strings.Contains(string(buf[:n]), "getstatus") {
			return
		}
		resp := []byte("\xff\xff\xff\xffstatusResponse\n\\mapname\\q3dm1\n")
		_, _ = pc.WriteTo(resp, addr)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m := &models.Monitor{URL: pc.LocalAddr().String(), GameDigGame: "quake3"}
	res := (&GameDigChecker{}).Check(ctx, m)
	if res.Status != 1 {
		t.Fatalf("status=%d msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "GameDig(Quake3) OK") {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestBrowserChecker_InvalidProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	m := &models.Monitor{URL: "file:///tmp/test.html"}
	res := (&BrowserChecker{}).Check(ctx, m)
	if res.Status != 0 {
		t.Fatalf("status=%d msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "only http and https") {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestGameDigChecker_UnsupportedGame(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	m := &models.Monitor{URL: "127.0.0.1:1234", GameDigGame: "unknown"}
	res := (&GameDigChecker{}).Check(ctx, m)
	if res.Status != 0 {
		t.Fatalf("status=%d msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "unsupported") {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestReadNulString(t *testing.T) {
	s, rest, err := readNulString([]byte("abc\x00def"))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if s != "abc" || string(rest) != "def" {
		t.Fatalf("got (%q,%q)", s, string(rest))
	}
	_, _, err = readNulString([]byte("abc"))
	if err == nil {
		t.Fatal("expected missing terminator error")
	}
	_ = fmt.Sprintf("%v", err)
}
