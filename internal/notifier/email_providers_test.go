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

func TestSendGridProvider_Send_Roundtrip(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	orig := sendgridAPIURL
	sendgridAPIURL = srv.URL
	defer func() { sendgridAPIURL = orig }()

	err := (&SendGridProvider{}).Send(context.Background(), map[string]string{
		"api_key": "SG.testkey",
		"from":    "from@example.com",
		"to":      "to@example.com",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		t.Errorf("expected Bearer auth header, got %q", authHeader)
	}
}

func TestSendGridProvider_MissingAPIKey(t *testing.T) {
	err := (&SendGridProvider{}).Send(context.Background(), map[string]string{
		"from": "from@example.com",
		"to":   "to@example.com",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

func TestSendGridProvider_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	orig := sendgridAPIURL
	sendgridAPIURL = srv.URL
	defer func() { sendgridAPIURL = orig }()

	err := (&SendGridProvider{}).Send(context.Background(), map[string]string{
		"api_key": "k", "from": "a@b.com", "to": "c@d.com",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestResendProvider_Send_Roundtrip(t *testing.T) {
	var gotPayload map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("expected Bearer Authorization header")
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := resendAPIURL
	resendAPIURL = srv.URL
	defer func() { resendAPIURL = orig }()

	err := (&ResendProvider{}).Send(context.Background(), map[string]string{
		"api_key": "re_testkey",
		"from":    "from@example.com",
		"to":      "to@example.com",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPayload["from"] != "from@example.com" {
		t.Errorf("expected from=from@example.com, got %v", gotPayload["from"])
	}
}

func TestResendProvider_MissingTo(t *testing.T) {
	err := (&ResendProvider{}).Send(context.Background(), map[string]string{
		"api_key": "key",
		"from":    "from@example.com",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing to")
	}
}

func TestTwilioProvider_Send_Roundtrip(t *testing.T) {
	var gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		u, _, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth")
		}
		gotUser = u
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	orig := twilioAPIBaseURL
	twilioAPIBaseURL = srv.URL
	defer func() { twilioAPIBaseURL = orig }()

	err := (&TwilioProvider{}).Send(context.Background(), map[string]string{
		"account_sid": "ACabc123",
		"auth_token":  "authtok456",
		"from":        "+15551234567",
		"to":          "+15559876543",
	}, Event{MonitorName: "watchdog", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUser != "ACabc123" {
		t.Errorf("expected Basic Auth user=ACabc123, got %q", gotUser)
	}
}

func TestTwilioProvider_MissingTo(t *testing.T) {
	err := (&TwilioProvider{}).Send(context.Background(), map[string]string{
		"account_sid": "AC123",
		"auth_token":  "tok",
		"from":        "+15551234567",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing to")
	}
}

func TestTwilioProvider_MissingAccountSID(t *testing.T) {
	err := (&TwilioProvider{}).Send(context.Background(), map[string]string{
		"auth_token": "tok",
		"from":       "+1",
		"to":         "+2",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing account_sid")
	}
}
