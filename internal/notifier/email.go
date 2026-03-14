package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// EmailProvider sends notifications via SMTP.
type EmailProvider struct{}

// Send sends an email alert.
// Required config fields: host, port, username, password, from, to
// Optional config fields: tls (values: "true"/"false", default "true")
func (p *EmailProvider) Send(_ context.Context, cfg map[string]string, e Event) error {
	host, err := RequiredField(cfg, "host")
	if err != nil {
		return err
	}
	port, err := RequiredField(cfg, "port")
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
	from, err := RequiredField(cfg, "from")
	if err != nil {
		return err
	}
	to, err := RequiredField(cfg, "to")
	if err != nil {
		return err
	}

	icon := "✅ UP"
	if e.Status == 0 {
		icon = "🔴 DOWN"
	}

	subject := fmt.Sprintf("[Service Monitor] %s is %s", e.MonitorName, e.StatusText())
	body := fmt.Sprintf(
		"Monitor: %s\nStatus: %s\nURL: %s\nLatency: %dms\nMessage: %s\nTime: %s\n\n-- Service Monitor",
		e.MonitorName, icon, e.MonitorURL, e.LatencyMs, e.Message,
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	)

	toList := strings.Split(to, ",")
	for i := range toList {
		toList[i] = strings.TrimSpace(toList[i])
	}

	msg := buildMIMEMessage(from, toList, subject, body)

	addr := net.JoinHostPort(host, port)
	auth := smtp.PlainAuth("", username, password, host)

	useTLS := cfg["tls"] != "false"

	if useTLS {
		tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("email: TLS dial: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("email: SMTP client: %w", err)
		}
		defer client.Close()

		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
		if err := client.Mail(from); err != nil {
			return fmt.Errorf("email: MAIL FROM: %w", err)
		}
		for _, rcpt := range toList {
			if err := client.Rcpt(rcpt); err != nil {
				return fmt.Errorf("email: RCPT TO %s: %w", rcpt, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("email: DATA: %w", err)
		}
		if _, err := fmt.Fprint(w, msg); err != nil {
			return fmt.Errorf("email: write: %w", err)
		}
		return w.Close()
	}

	// Plain SMTP with STARTTLS
	return smtp.SendMail(addr, auth, from, toList, []byte(msg))
}

func buildMIMEMessage(from string, to []string, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}
