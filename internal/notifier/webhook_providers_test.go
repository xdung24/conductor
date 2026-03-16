package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// webhookProviderSendTest is a table-driven helper for URL-webhook providers.
func webhookProviderSendTest(t *testing.T, p Provider) {
	t.Helper()

	t.Run("OK", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		err := p.Send(context.Background(), map[string]string{"url": srv.URL}, Event{
			MonitorName: "My Service",
			MonitorURL:  "https://example.com",
			Status:      1,
			LatencyMs:   5,
			Message:     "200 OK",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("MissingURL", func(t *testing.T) {
		err := p.Send(context.Background(), map[string]string{}, Event{})
		if err == nil {
			t.Fatal("expected error for missing url")
		}
	})

	t.Run("ServerError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		err := p.Send(context.Background(), map[string]string{"url": srv.URL}, Event{Status: 1})
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})
}

func TestMattermostProvider(t *testing.T) {
	webhookProviderSendTest(t, &MattermostProvider{})
}

func TestRocketChatProvider(t *testing.T) {
	webhookProviderSendTest(t, &RocketChatProvider{})
}

func TestDingDingProvider(t *testing.T) {
	webhookProviderSendTest(t, &DingDingProvider{})
}

func TestFeishuProvider(t *testing.T) {
	webhookProviderSendTest(t, &FeishuProvider{})
}

func TestGoogleChatProvider(t *testing.T) {
	webhookProviderSendTest(t, &GoogleChatProvider{})
}

func TestTeamsProvider(t *testing.T) {
	webhookProviderSendTest(t, &TeamsProvider{})
}

func TestWeComProvider(t *testing.T) {
	webhookProviderSendTest(t, &WeComProvider{})
}

func TestYZJProvider(t *testing.T) {
	webhookProviderSendTest(t, &YZJProvider{})
}

func TestLunaSeaProvider(t *testing.T) {
	webhookProviderSendTest(t, &LunaSeaProvider{})
}
