package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPagerDutyProvider_Trigger(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	orig := pagerdutyEventsURL
	pagerdutyEventsURL = srv.URL
	defer func() { pagerdutyEventsURL = orig }()

	err := (&PagerDutyProvider{}).Send(context.Background(), map[string]string{
		"routing_key": "rk-abc123",
	}, Event{MonitorID: 7, MonitorName: "svc", Status: 0, LatencyMs: 300, Message: "timeout"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["event_action"] != "trigger" {
		t.Errorf("expected event_action=trigger, got %v", gotBody["event_action"])
	}
	if gotBody["routing_key"] != "rk-abc123" {
		t.Errorf("expected routing_key=rk-abc123, got %v", gotBody["routing_key"])
	}
}

func TestPagerDutyProvider_Resolve(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	orig := pagerdutyEventsURL
	pagerdutyEventsURL = srv.URL
	defer func() { pagerdutyEventsURL = orig }()

	_ = (&PagerDutyProvider{}).Send(context.Background(), map[string]string{
		"routing_key": "rk-abc",
	}, Event{Status: 1})
	if gotBody["event_action"] != "resolve" {
		t.Errorf("expected event_action=resolve, got %v", gotBody["event_action"])
	}
}

func TestPagerDutyProvider_MissingRoutingKey(t *testing.T) {
	err := (&PagerDutyProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing routing_key")
	}
}

func TestPagerDutyProvider_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	orig := pagerdutyEventsURL
	pagerdutyEventsURL = srv.URL
	defer func() { pagerdutyEventsURL = orig }()

	err := (&PagerDutyProvider{}).Send(context.Background(), map[string]string{"routing_key": "k"}, Event{})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestPushoverProvider_Send(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.Form
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := pushoverAPIURL
	pushoverAPIURL = srv.URL
	defer func() { pushoverAPIURL = orig }()

	err := (&PushoverProvider{}).Send(context.Background(), map[string]string{
		"user_key":  "uk123",
		"api_token": "tok456",
		"device":    "myiphone",
	}, Event{MonitorName: "svc", MonitorURL: "https://example.com", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotForm.Get("user") != "uk123" {
		t.Errorf("expected user=uk123, got %v", gotForm.Get("user"))
	}
	if gotForm.Get("device") != "myiphone" {
		t.Errorf("expected device=myiphone, got %v", gotForm.Get("device"))
	}
	if gotForm.Get("priority") != "1" {
		t.Errorf("expected priority=1 (DOWN), got %v", gotForm.Get("priority"))
	}
}

func TestPushoverProvider_MissingUserKey(t *testing.T) {
	err := (&PushoverProvider{}).Send(context.Background(), map[string]string{"api_token": "tok"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing user_key")
	}
}

func TestPushoverProvider_MissingAPIToken(t *testing.T) {
	err := (&PushoverProvider{}).Send(context.Background(), map[string]string{"user_key": "uk"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing api_token")
	}
}

func TestMatrixProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/_matrix/client/v3/rooms/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&MatrixProvider{}).Send(context.Background(), map[string]string{
		"homeserver_url": srv.URL,
		"access_token":   "mytoken",
		"room_id":        "!roomid:localhost",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatrixProvider_MissingField(t *testing.T) {
	err := (&MatrixProvider{}).Send(context.Background(), map[string]string{
		"homeserver_url": "http://x",
		"access_token":   "tok",
		// room_id missing
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing room_id")
	}
}

func TestSignalProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/send" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	err := (&SignalProvider{}).Send(context.Background(), map[string]string{
		"url":        srv.URL,
		"number":     "+1234567890",
		"recipients": "+0987654321,+1111111111",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWAHAProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sendText" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&WAHAProvider{}).Send(context.Background(), map[string]string{
		"url":   srv.URL,
		"phone": "1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWhapiProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := whapiAPIURL
	whapiAPIURL = srv.URL
	defer func() { whapiAPIURL = orig }()

	err := (&WhapiProvider{}).Send(context.Background(), map[string]string{
		"token": "mytoken",
		"phone": "+1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWhapiProvider_MissingToken(t *testing.T) {
	err := (&WhapiProvider{}).Send(context.Background(), map[string]string{"phone": "+1"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestOneSenderProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&OneSenderProvider{}).Send(context.Background(), map[string]string{
		"url":   srv.URL,
		"token": "tok",
		"phone": "+1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvolutionProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&EvolutionProvider{}).Send(context.Background(), map[string]string{
		"url":      srv.URL,
		"api_key":  "key",
		"instance": "my-instance",
		"phone":    "1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
