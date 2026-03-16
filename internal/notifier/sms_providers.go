package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	elksAPIURL           = "https://api.46elks.com/a1/sms"
	brevoSMSAPIURL       = "https://api.brevo.com/v3/transactionalSMS/sms"
	callMeBotSignalURL   = "https://api.callmebot.com/signal/send.php"
	callMeBotWhatsAppURL = "https://api.callmebot.com/whatsapp.php"
	cellsyntAPIURL       = "https://se-1.cellsynt.net/sms.php"
	freeMobileAPIURL     = "https://smsapi.free-mobile.fr/sendmsg"
	gtxMessagingAPIURL   = "https://api.gtx-messaging.net/smsc/1/messages"
	octopushAPIURL       = "https://api.octopush.com/v1/public/sms-campaign/send-one-time"
)

// ElksProvider sends SMS via the 46elks API.
type ElksProvider struct{}

// Send sends an SMS via 46elks.
// Required config fields: username, password, from, to
func (p *ElksProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	username, err := RequiredField(cfg, "username")
	if err != nil {
		return err
	}
	password, err := RequiredField(cfg, "password")
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

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms. %s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	form := url.Values{}
	form.Set("from", from)
	form.Set("to", to)
	form.Set("message", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, elksAPIURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("46elks: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "service-monitor/1.0")
	req.SetBasicAuth(username, password)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("46elks: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("46elks: server returned %d", resp.StatusCode)
	}
	return nil
}

// BrevoProvider sends transactional SMS via the Brevo (formerly Sendinblue) API.
type BrevoProvider struct{}

// Send sends an SMS via Brevo.
// Required config fields: api_key, sender_name, sender_sms, to
func (p *BrevoProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	apiKey, err := RequiredField(cfg, "api_key")
	if err != nil {
		return err
	}
	senderName, err := RequiredField(cfg, "sender_name")
	if err != nil {
		return err
	}
	senderSMS, err := RequiredField(cfg, "sender_sms")
	if err != nil {
		return err
	}
	to, err := RequiredField(cfg, "to")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms. %s",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs, e.Message)

	payload := map[string]interface{}{
		"sender":    map[string]string{"name": senderName, "phone": senderSMS},
		"recipient": to,
		"content":   message,
		"type":      "transactional",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("brevo: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoSMSAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("brevo: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("brevo: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("brevo: server returned %d", resp.StatusCode)
	}
	return nil
}

// CallMeBotProvider sends WhatsApp or Signal messages via the CallMeBot API.
type CallMeBotProvider struct{}

// Send sends a message via CallMeBot.
// Required config fields: phone, apikey
// Optional config fields: type ("whatsapp" or "signal", default "whatsapp")
func (p *CallMeBotProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	phone, err := RequiredField(cfg, "phone")
	if err != nil {
		return err
	}
	apiKey, err := RequiredField(cfg, "apikey")
	if err != nil {
		return err
	}

	msgType := cfg["type"]
	if msgType != "signal" {
		msgType = "whatsapp"
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	var apiURL string
	params := url.Values{}
	if msgType == "signal" {
		apiURL = callMeBotSignalURL
		params.Set("phone", phone)
		params.Set("apikey", apiKey)
		params.Set("text", message)
	} else {
		apiURL = callMeBotWhatsAppURL
		params.Set("phone", phone)
		params.Set("apikey", apiKey)
		params.Set("text", message)
	}

	fullURL := apiURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("callmebot: create request: %w", err)
	}
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("callmebot: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callmebot: server returned %d", resp.StatusCode)
	}
	return nil
}

// CellsyntProvider sends SMS via the Cellsynt API.
type CellsyntProvider struct{}

// Send sends an SMS via Cellsynt.
// Required config fields: username, password, to, from
func (p *CellsyntProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	username, err := RequiredField(cfg, "username")
	if err != nil {
		return err
	}
	password, err := RequiredField(cfg, "password")
	if err != nil {
		return err
	}
	to, err := RequiredField(cfg, "to")
	if err != nil {
		return err
	}
	from, err := RequiredField(cfg, "from")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	params := url.Values{}
	params.Set("username", username)
	params.Set("password", password)
	params.Set("destination", to)
	params.Set("origintype", "alpha")
	params.Set("origin", from)
	params.Set("type", "text")
	params.Set("charset", "UTF-8")
	params.Set("message", message)

	fullURL := cellsyntAPIURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("cellsynt: create request: %w", err)
	}
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cellsynt: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cellsynt: server returned %d", resp.StatusCode)
	}
	return nil
}

// FreeMobileProvider sends SMS via the Free Mobile (France) API.
type FreeMobileProvider struct{}

// Send sends an SMS via Free Mobile.
// Required config fields: user, pass
func (p *FreeMobileProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	user, err := RequiredField(cfg, "user")
	if err != nil {
		return err
	}
	pass, err := RequiredField(cfg, "pass")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	params := url.Values{}
	params.Set("user", user)
	params.Set("pass", pass)
	params.Set("msg", message)

	fullURL := freeMobileAPIURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("freemobile: create request: %w", err)
	}
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("freemobile: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("freemobile: server returned %d", resp.StatusCode)
	}
	return nil
}

// GTXMessagingProvider sends SMS via the GTX Messaging API.
type GTXMessagingProvider struct{}

// Send sends an SMS via GTX Messaging.
// Required config fields: username, password, from, to
func (p *GTXMessagingProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	username, err := RequiredField(cfg, "username")
	if err != nil {
		return err
	}
	password, err := RequiredField(cfg, "password")
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

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	payload := map[string]interface{}{
		"Originator": from,
		"Recipients": []string{to},
		"Body":       message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gtxmessaging: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gtxMessagingAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gtxmessaging: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")
	req.SetBasicAuth(username, password)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gtxmessaging: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gtxmessaging: server returned %d", resp.StatusCode)
	}
	return nil
}

// OctopushProvider sends SMS via the Octopush API.
type OctopushProvider struct{}

// Send sends an SMS via Octopush.
// Required config fields: api_key, api_login, recipients (comma-separated), sender
func (p *OctopushProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	apiKey, err := RequiredField(cfg, "api_key")
	if err != nil {
		return err
	}
	apiLogin, err := RequiredField(cfg, "api_login")
	if err != nil {
		return err
	}
	recipientsStr, err := RequiredField(cfg, "recipients")
	if err != nil {
		return err
	}
	sender, err := RequiredField(cfg, "sender")
	if err != nil {
		return err
	}

	rawRecipients := strings.Split(recipientsStr, ",")
	recipients := make([]string, 0, len(rawRecipients))
	for _, r := range rawRecipients {
		r = strings.TrimSpace(r)
		if r != "" {
			recipients = append(recipients, r)
		}
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	payload := map[string]interface{}{
		"recipients": recipients,
		"text":       message,
		"type":       "sms",
		"sender":     sender,
		"purpose":    "alert",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("octopush: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, octopushAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("octopush: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("api-login", apiLogin)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("octopush: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("octopush: server returned %d", resp.StatusCode)
	}
	return nil
}
