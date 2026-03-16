package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGotifyProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/message") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("token") == "" {
			t.Error("expected token query param")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := &GotifyProvider{}
	err := p.Send(context.Background(), map[string]string{"url": srv.URL, "token": "abc123"}, Event{
		MonitorName: "My Service",
		Status:      1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGotifyProvider_MissingURL(t *testing.T) {
	err := (&GotifyProvider{}).Send(context.Background(), map[string]string{"token": "abc"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestGotifyProvider_MissingToken(t *testing.T) {
	err := (&GotifyProvider{}).Send(context.Background(), map[string]string{"url": "http://x"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestServerChanProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "mysendkey") {
			t.Errorf("expected send_key in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := serverChanBaseURL
	serverChanBaseURL = srv.URL
	defer func() { serverChanBaseURL = orig }()

	err := (&ServerChanProvider{}).Send(context.Background(), map[string]string{
		"send_key": "mysendkey",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerChanProvider_MissingSendKey(t *testing.T) {
	err := (&ServerChanProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing send_key")
	}
}

func TestPushPlusProvider_Send(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := pushPlusAPIURL
	pushPlusAPIURL = srv.URL
	defer func() { pushPlusAPIURL = orig }()

	err := (&PushPlusProvider{}).Send(context.Background(), map[string]string{
		"token": "mypushplustoken",
		"topic": "testchannel",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["token"] != "mypushplustoken" {
		t.Errorf("expected token=mypushplustoken, got %v", gotBody["token"])
	}
	if gotBody["topic"] != "testchannel" {
		t.Errorf("expected topic=testchannel, got %v", gotBody["topic"])
	}
}

func TestPushPlusProvider_MissingToken(t *testing.T) {
	err := (&PushPlusProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestLINEProvider_Send(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := lineNotifyAPIURL
	lineNotifyAPIURL = srv.URL
	defer func() { lineNotifyAPIURL = orig }()

	err := (&LINEProvider{}).Send(context.Background(), map[string]string{
		"token": "linenotifytoken",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authHeader != "Bearer linenotifytoken" {
		t.Errorf("expected Authorization=Bearer linenotifytoken, got %q", authHeader)
	}
}

func TestLINEProvider_MissingToken(t *testing.T) {
	err := (&LINEProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestBarkProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/my-device-key") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&BarkProvider{}).Send(context.Background(), map[string]string{
		"device_key": "my-device-key",
		"server_url": srv.URL,
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBarkProvider_MissingKey(t *testing.T) {
	err := (&BarkProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing device_key")
	}
}

func TestGorushProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/push" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&GorushProvider{}).Send(context.Background(), map[string]string{
		"server_url": srv.URL,
		"tokens":     "tok1,tok2",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGorushProvider_MissingServerURL(t *testing.T) {
	err := (&GorushProvider{}).Send(context.Background(), map[string]string{"tokens": "tok"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing server_url")
	}
}

func TestHomeAssistantProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/services/notify/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&HomeAssistantProvider{}).Send(context.Background(), map[string]string{
		"url":             srv.URL,
		"token":           "mytoken",
		"notification_id": "mobile_app",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOneBotProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&OneBotProvider{}).Send(context.Background(), map[string]string{
		"url": srv.URL,
		"to":  "12345678",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
