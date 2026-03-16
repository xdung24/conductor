package notifier

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var twilioAPIBaseURL = "https://api.twilio.com/2010-04-01/Accounts"

// TwilioProvider sends SMS or voice messages via the Twilio REST API.
type TwilioProvider struct{}

// Send sends an SMS message via Twilio.
// Required config fields: account_sid, auth_token, from (Twilio number), to (recipient number)
func (p *TwilioProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	accountSID, err := RequiredField(cfg, "account_sid")
	if err != nil {
		return err
	}
	authToken, err := RequiredField(cfg, "auth_token")
	if err != nil {
		return err
	}
	from, err := RequiredField(cfg, "from")
	if err != nil {
		return err
	}
	to, err := RequiredField(cfg, "to")
	if err != nil {
		return err
	}

	body := fmt.Sprintf("[%s] %s is %s\nURL: %s\nLatency: %d ms\n%s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	form := url.Values{}
	form.Set("From", from)
	form.Set("To", to)
	form.Set("Body", body)

	endpoint := fmt.Sprintf("%s/%s/Messages.json", twilioAPIBaseURL, accountSID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "service-monitor/1.0")
	req.SetBasicAuth(accountSID, authToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio: server returned %d", resp.StatusCode)
	}
	return nil
}
