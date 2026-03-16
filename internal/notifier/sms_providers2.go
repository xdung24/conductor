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
	promoSMSAPIURL  = "https://api.promosms.com/api/sms/send"
	serwerSMSAPIURL = "https://api1.serwersms.pl/zdalnie/sms/send_text"
	sevenIOAPIURL   = "https://gateway.sms77.io/api/sms"
	smscAPIURL      = "https://smsc.ru/sys/send.php"
	smsIrAPIURL     = "https://api.sms.ir/v1/send"
)

// PromoSMSProvider sends SMS via the PromoSMS API.
type PromoSMSProvider struct{}

// Send sends an SMS via PromoSMS.
// Required config fields: login, password, from, to
func (p *PromoSMSProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	login, err := RequiredField(cfg, "login")
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

	params := url.Values{}
	params.Set("login", login)
	params.Set("password", password)
	params.Set("from", from)
	params.Set("to", to)
	params.Set("text", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, promoSMSAPIURL,
		strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("promosms: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("promosms: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("promosms: server returned %d", resp.StatusCode)
	}
	return nil
}

// SerwerSMSProvider sends SMS via the SerwerSMS (Poland) API.
type SerwerSMSProvider struct{}

// Send sends an SMS via SerwerSMS.
// Required config fields: username, password, from, to
func (p *SerwerSMSProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
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
		"username": username,
		"password": password,
		"sender":   from,
		"phone":    to,
		"message":  message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serwersms: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serwerSMSAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("serwersms: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("serwersms: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("serwersms: server returned %d", resp.StatusCode)
	}
	return nil
}

// SevenIOProvider sends SMS via the seven.io (sms77) API.
type SevenIOProvider struct{}

// Send sends an SMS via seven.io.
// Required config fields: api_key, from, to
func (p *SevenIOProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	apiKey, err := RequiredField(cfg, "api_key")
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

	form := url.Values{}
	form.Set("from", from)
	form.Set("to", to)
	form.Set("text", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sevenIOAPIURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("sevenio: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("SentWith", "service-monitor")
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sevenio: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sevenio: server returned %d", resp.StatusCode)
	}
	return nil
}

// SMSCProvider sends SMS via the SMSC.ru API.
type SMSCProvider struct{}

// Send sends an SMS via SMSC.
// Required config fields: login, password, phones (comma-separated)
func (p *SMSCProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	login, err := RequiredField(cfg, "login")
	if err != nil {
		return err
	}
	password, err := RequiredField(cfg, "password")
	if err != nil {
		return err
	}
	phones, err := RequiredField(cfg, "phones")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	params := url.Values{}
	params.Set("login", login)
	params.Set("psw", password)
	params.Set("phones", phones)
	params.Set("mes", message)
	params.Set("fmt", "3") // JSON response

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, smscAPIURL,
		strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("smsc: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("smsc: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("smsc: server returned %d", resp.StatusCode)
	}
	return nil
}

// SMSEagleProvider sends SMS via an SMSEagle hardware device.
type SMSEagleProvider struct{}

// Send sends an SMS via SMSEagle.
// Required config fields: url (device URL), login, password, to
func (p *SMSEagleProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	deviceURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	login, err := RequiredField(cfg, "login")
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

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	params := url.Values{}
	params.Set("login", login)
	params.Set("pass", password)
	params.Set("to", to)
	params.Set("message", message)

	endpoint := strings.TrimRight(deviceURL, "/") + "/index.php/http_api/send_sms?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("smseagle: create request: %w", err)
	}
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("smseagle: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("smseagle: server returned %d", resp.StatusCode)
	}
	return nil
}

// SMSIrProvider sends SMS via the SMS.ir (Iran) API.
type SMSIrProvider struct{}

// Send sends an SMS via SMS.ir.
// Required config fields: api_key, line_number, mobile
func (p *SMSIrProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	apiKey, err := RequiredField(cfg, "api_key")
	if err != nil {
		return err
	}
	lineNumber, err := RequiredField(cfg, "line_number")
	if err != nil {
		return err
	}
	mobile, err := RequiredField(cfg, "mobile")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	payload := map[string]interface{}{
		"lineNumber":  lineNumber,
		"mobiles":     []string{mobile},
		"messageText": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("smsir: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, smsIrAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("smsir: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("User-Agent", "service-monitor/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("smsir: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("smsir: server returned %d", resp.StatusCode)
	}
	return nil
}

// TeltonikaProvider sends SMS via a Teltonika router SMS API.
type TeltonikaProvider struct{}

// Send sends an SMS via a Teltonika router.
// Required config fields: url (router URL), username, password, phone
func (p *TeltonikaProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	routerURL, err := RequiredField(cfg, "url")
	if err != nil {
		return err
	}
	username, err := RequiredField(cfg, "username")
	if err != nil {
		return err
	}
	password, err := RequiredField(cfg, "password")
	if err != nil {
		return err
	}
	phone, err := RequiredField(cfg, "phone")
	if err != nil {
		return err
	}

	message := fmt.Sprintf("[%s] %s is %s. URL: %s. Latency: %d ms.",
		e.StatusText(), e.MonitorName, e.StatusText(), e.MonitorURL, e.LatencyMs)

	payload := map[string]interface{}{
		"number":  phone,
		"message": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("teltonika: marshal payload: %w", err)
	}

	endpoint := strings.TrimRight(routerURL, "/") + "/api/sms/send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("teltonika: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "service-monitor/1.0")
	req.SetBasicAuth(username, password)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("teltonika: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("teltonika: server returned %d", resp.StatusCode)
	}
	return nil
}
