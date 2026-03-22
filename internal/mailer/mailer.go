package mailer

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"

	"github.com/xdung24/conductor/internal/config"
)

// Mailer sends transactional system emails via SMTP.
// When Host is empty all send calls are no-ops (system email is disabled).
type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
	useTLS   bool
	bcc      string
}

// New builds a Mailer from configuration.
// Returns a no-op mailer (Enabled() == false) when cfg.SystemSMTPHost is empty.
func New(cfg *config.Config) *Mailer {
	return &Mailer{
		host:     cfg.SystemSMTPHost,
		port:     cfg.SystemSMTPPort,
		username: cfg.SystemSMTPUsername,
		password: cfg.SystemSMTPPassword,
		from:     cfg.SystemSMTPFrom,
		useTLS:   cfg.SystemSMTPTLS != "false",
		bcc:      cfg.SystemSMTPBCC,
	}
}

// Enabled reports whether system email is configured.
func (m *Mailer) Enabled() bool {
	return m.host != ""
}

// SendAsync sends an email in a background goroutine.
// Errors are logged as warnings and never surface to the caller.
// If the mailer is not enabled this is a no-op.
func (m *Mailer) SendAsync(to, subject, htmlBody string) {
	if !m.Enabled() {
		return
	}
	go func() {
		if err := m.send(to, subject, htmlBody); err != nil {
			slog.Warn("system email failed", "to", to, "error", err)
		}
	}()
}

// send delivers a single email synchronously.
func (m *Mailer) send(to, subject, htmlBody string) error {
	addr := net.JoinHostPort(m.host, m.port)

	// Build recipient list (primary + optional BCC).
	rcpts := []string{to}
	if m.bcc != "" && m.bcc != to {
		rcpts = append(rcpts, m.bcc)
	}

	// Compose RFC 2822 message.
	var sb strings.Builder
	sb.WriteString("From: " + m.from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	if m.bcc != "" {
		sb.WriteString("Bcc: " + m.bcc + "\r\n")
	}
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: multipart/alternative; boundary=\"_boundary_conductor\"\r\n")
	sb.WriteString("\r\n")

	// Plain-text part (strip tags naively).
	plainText := stripHTML(htmlBody)
	sb.WriteString("--_boundary_conductor\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	sb.WriteString(plainText + "\r\n")

	// HTML part.
	sb.WriteString("--_boundary_conductor\r\n")
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	sb.WriteString(htmlBody + "\r\n")
	sb.WriteString("--_boundary_conductor--\r\n")

	msgBytes := []byte(sb.String())

	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	if m.useTLS {
		// STARTTLS (port 587 typical).
		c, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("dial smtp: %w", err)
		}
		defer c.Close()                           //nolint:errcheck
		tlsCfg := &tls.Config{ServerName: m.host} //nolint:gosec // user-configured host
		if err = c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
		if auth != nil {
			if err = c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err = c.Mail(m.from); err != nil {
			return fmt.Errorf("smtp MAIL FROM: %w", err)
		}
		for _, r := range rcpts {
			if err = c.Rcpt(r); err != nil {
				return fmt.Errorf("smtp RCPT TO %s: %w", r, err)
			}
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("smtp DATA: %w", err)
		}
		if _, err = w.Write(msgBytes); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return w.Close()
	}

	// Plain SMTP (port 25 or 465 with implicit TLS not handled here).
	return smtp.SendMail(addr, auth, m.from, rcpts, msgBytes)
}

// stripHTML removes HTML tags to produce a plain-text fallback.
func stripHTML(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			out.WriteRune(' ')
		case !inTag:
			out.WriteRune(r)
		}
	}
	// Collapse runs of whitespace.
	result := strings.Join(strings.Fields(out.String()), " ")
	return result
}
