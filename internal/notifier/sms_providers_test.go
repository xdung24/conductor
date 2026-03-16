package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- 46elks ---

func TestElksProvider_Send(t *testing.T) {
	var gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth")
		}
		gotUser = u
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := elksAPIURL
	elksAPIURL = srv.URL
	defer func() { elksAPIURL = orig }()

	err := (&ElksProvider{}).Send(context.Background(), map[string]string{
		"username": "user46", "password": "pw46", "from": "SVC", "to": "+1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUser != "user46" {
		t.Errorf("expected Basic Auth user=user46, got %q", gotUser)
	}
}

func TestElksProvider_MissingUsername(t *testing.T) {
	err := (&ElksProvider{}).Send(context.Background(), map[string]string{
		"password": "pw", "from": "SVC", "to": "+1",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing username")
	}
}

func TestElksProvider_MissingTo(t *testing.T) {
	err := (&ElksProvider{}).Send(context.Background(), map[string]string{
		"username": "u", "password": "p", "from": "SVC",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing to")
	}
}

// --- Brevo ---

func TestBrevoProvider_Send(t *testing.T) {
	var gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	orig := brevoSMSAPIURL
	brevoSMSAPIURL = srv.URL
	defer func() { brevoSMSAPIURL = orig }()

	err := (&BrevoProvider{}).Send(context.Background(), map[string]string{
		"api_key":     "brevo-key",
		"sender_name": "SVC",
		"sender_sms":  "+15550000000",
		"to":          "+15551234567",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAPIKey != "brevo-key" {
		t.Errorf("expected api-key=brevo-key, got %q", gotAPIKey)
	}
}

func TestBrevoProvider_MissingFields(t *testing.T) {
	err := (&BrevoProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

// --- CallMeBot ---

func TestCallMeBotProvider_Send_WhatsApp(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := callMeBotWhatsAppURL
	callMeBotWhatsAppURL = srv.URL
	defer func() { callMeBotWhatsAppURL = orig }()

	err := (&CallMeBotProvider{}).Send(context.Background(), map[string]string{
		"phone": "+1234567890", "apikey": "cbkey", "type": "whatsapp",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery == "" {
		t.Error("expected query string in request")
	}
}

func TestCallMeBotProvider_Send_Signal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := callMeBotSignalURL
	callMeBotSignalURL = srv.URL
	defer func() { callMeBotSignalURL = orig }()

	err := (&CallMeBotProvider{}).Send(context.Background(), map[string]string{
		"phone": "+1234567890", "apikey": "cbkey", "type": "signal",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallMeBotProvider_MissingPhone(t *testing.T) {
	err := (&CallMeBotProvider{}).Send(context.Background(), map[string]string{"apikey": "k"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing phone")
	}
}

// --- Cellsynt ---

func TestCellsyntProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") == "" {
			t.Error("expected username query param")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := cellsyntAPIURL
	cellsyntAPIURL = srv.URL
	defer func() { cellsyntAPIURL = orig }()

	err := (&CellsyntProvider{}).Send(context.Background(), map[string]string{
		"username": "celluser", "password": "cellpw", "from": "SVC", "to": "+46701234567",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCellsyntProvider_MissingPassword(t *testing.T) {
	err := (&CellsyntProvider{}).Send(context.Background(), map[string]string{
		"username": "u", "to": "+1", "from": "SVC",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing password")
	}
}

// --- FreeMobile ---

func TestFreeMobileProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user") == "" {
			t.Error("expected user query param")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := freeMobileAPIURL
	freeMobileAPIURL = srv.URL
	defer func() { freeMobileAPIURL = orig }()

	err := (&FreeMobileProvider{}).Send(context.Background(), map[string]string{
		"user": "freeuser", "pass": "freepw",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFreeMobileProvider_MissingUser(t *testing.T) {
	err := (&FreeMobileProvider{}).Send(context.Background(), map[string]string{"pass": "p"}, Event{})
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

// --- GTX Messaging ---

func TestGTXMessagingProvider_Send(t *testing.T) {
	var gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth")
		}
		gotUser = u
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := gtxMessagingAPIURL
	gtxMessagingAPIURL = srv.URL
	defer func() { gtxMessagingAPIURL = orig }()

	err := (&GTXMessagingProvider{}).Send(context.Background(), map[string]string{
		"username": "gtxuser", "password": "gtxpw", "from": "SVC", "to": "+491234567890",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUser != "gtxuser" {
		t.Errorf("expected Basic Auth user=gtxuser, got %q", gotUser)
	}
}

func TestGTXMessagingProvider_MissingTo(t *testing.T) {
	err := (&GTXMessagingProvider{}).Send(context.Background(), map[string]string{
		"username": "u", "password": "p", "from": "SVC",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing to")
	}
}

// --- Octopush ---

func TestOctopushProvider_Send(t *testing.T) {
	var gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := octopushAPIURL
	octopushAPIURL = srv.URL
	defer func() { octopushAPIURL = orig }()

	err := (&OctopushProvider{}).Send(context.Background(), map[string]string{
		"api_key":    "octo-key",
		"api_login":  "octo-login",
		"recipients": "+33612345678",
		"sender":     "SVC",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAPIKey != "octo-key" {
		t.Errorf("expected api-key=octo-key, got %q", gotAPIKey)
	}
}

func TestOctopushProvider_MissingFields(t *testing.T) {
	err := (&OctopushProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

// --- PromoSMS ---

func TestPromoSMSProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("login") == "" {
			t.Error("expected login field")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := promoSMSAPIURL
	promoSMSAPIURL = srv.URL
	defer func() { promoSMSAPIURL = orig }()

	err := (&PromoSMSProvider{}).Send(context.Background(), map[string]string{
		"login": "promologin", "password": "promopw", "from": "SVC", "to": "+48600100200",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromoSMSProvider_MissingFields(t *testing.T) {
	err := (&PromoSMSProvider{}).Send(context.Background(), map[string]string{}, Event{})
	if err == nil {
		t.Fatal("expected error for missing login")
	}
}

// --- SerwerSMS ---

func TestSerwerSMSProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := serwerSMSAPIURL
	serwerSMSAPIURL = srv.URL
	defer func() { serwerSMSAPIURL = orig }()

	err := (&SerwerSMSProvider{}).Send(context.Background(), map[string]string{
		"username": "swuser", "password": "swpw", "from": "SVC", "to": "+48600100200",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSerwerSMSProvider_MissingTo(t *testing.T) {
	err := (&SerwerSMSProvider{}).Send(context.Background(), map[string]string{
		"username": "u", "password": "p", "from": "SVC",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing to")
	}
}

// --- SevenIO ---

func TestSevenIOProvider_Send(t *testing.T) {
	var gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := sevenIOAPIURL
	sevenIOAPIURL = srv.URL
	defer func() { sevenIOAPIURL = orig }()

	err := (&SevenIOProvider{}).Send(context.Background(), map[string]string{
		"api_key": "sevenkey", "from": "+1234567890", "to": "+0987654321",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAPIKey != "sevenkey" {
		t.Errorf("expected X-Api-Key=sevenkey, got %q", gotAPIKey)
	}
}

func TestSevenIOProvider_MissingAPIKey(t *testing.T) {
	err := (&SevenIOProvider{}).Send(context.Background(), map[string]string{
		"from": "+1", "to": "+2",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

// --- SMSC ---

func TestSMSCProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("login") == "" {
			t.Error("expected login field")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := smscAPIURL
	smscAPIURL = srv.URL
	defer func() { smscAPIURL = orig }()

	err := (&SMSCProvider{}).Send(context.Background(), map[string]string{
		"login": "smsclogin", "password": "smscpw", "phones": "+79161234567",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSMSCProvider_MissingLogin(t *testing.T) {
	err := (&SMSCProvider{}).Send(context.Background(), map[string]string{
		"password": "p", "phones": "+1",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing login")
	}
}

// --- SMSEagle ---

func TestSMSEagleProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("login") == "" {
			t.Error("expected login query param")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&SMSEagleProvider{}).Send(context.Background(), map[string]string{
		"url": srv.URL, "login": "eaglelogin", "password": "eaglepw", "to": "+1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSMSEagleProvider_MissingURL(t *testing.T) {
	err := (&SMSEagleProvider{}).Send(context.Background(), map[string]string{
		"login": "l", "password": "p", "to": "+1",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

// --- SMS.ir ---

func TestSMSIrProvider_Send(t *testing.T) {
	var gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := smsIrAPIURL
	smsIrAPIURL = srv.URL
	defer func() { smsIrAPIURL = orig }()

	err := (&SMSIrProvider{}).Send(context.Background(), map[string]string{
		"api_key": "smsirkey", "line_number": "30004820", "mobile": "+989121234567",
	}, Event{MonitorName: "svc", Status: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAPIKey != "smsirkey" {
		t.Errorf("expected x-api-key=smsirkey, got %q", gotAPIKey)
	}
}

func TestSMSIrProvider_MissingAPIKey(t *testing.T) {
	err := (&SMSIrProvider{}).Send(context.Background(), map[string]string{
		"line_number": "123", "mobile": "+1",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

// --- Teltonika ---

func TestTeltonikaProvider_Send(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := (&TeltonikaProvider{}).Send(context.Background(), map[string]string{
		"url": srv.URL, "username": "admin", "password": "admin", "phone": "+1234567890",
	}, Event{MonitorName: "svc", Status: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeltonikaProvider_MissingPhone(t *testing.T) {
	err := (&TeltonikaProvider{}).Send(context.Background(), map[string]string{
		"url": "http://router", "username": "u", "password": "p",
	}, Event{})
	if err == nil {
		t.Fatal("expected error for missing phone")
	}
}
