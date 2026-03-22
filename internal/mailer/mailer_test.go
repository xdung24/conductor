package mailer

import (
	"strings"
	"testing"

	"github.com/xdung24/conductor/internal/config"
)

// ---------------------------------------------------------------------------
// stripHTML
// ---------------------------------------------------------------------------

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "Hello World", "Hello World"},
		{"single tag", "<p>Hello</p>", "Hello"},
		{"nested tags", "<div><p>Hello <strong>World</strong></p></div>", "Hello World"},
		{"attributes", `<a href="http://example.com">Click me</a>`, "Click me"},
		{"whitespace collapse", "<p>  Hello   </p>  <p>  World  </p>", "Hello World"},
		{"self-closing", "<br/>text", "text"},
		{"style tag content stripped", "<style>body{color:red}</style>text", "body{color:red} text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Mailer.Enabled
// ---------------------------------------------------------------------------

func TestMailerEnabled(t *testing.T) {
	disabled := New(&config.Config{})
	if disabled.Enabled() {
		t.Error("expected Enabled() == false when SMTP host is empty")
	}

	enabled := New(&config.Config{SystemSMTPHost: "smtp.example.com"})
	if !enabled.Enabled() {
		t.Error("expected Enabled() == true when SMTP host is set")
	}
}

// SendAsync on a disabled mailer must be a no-op (no panic, no goroutine launch).
func TestSendAsyncNoopWhenDisabled(t *testing.T) {
	m := New(&config.Config{})
	m.SendAsync("to@example.com", "subject", "<p>body</p>") // must not panic
}

// ---------------------------------------------------------------------------
// Render* — template rendering
// ---------------------------------------------------------------------------

// assertEmail checks the common invariants for every rendered email.
func assertEmail(t *testing.T, name, html string) {
	t.Helper()
	if html == "" {
		t.Fatalf("%s: rendered HTML is empty", name)
	}
	if strings.Contains(html, "{{") {
		t.Errorf("%s: rendered HTML contains unresolved template syntax", name)
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Errorf("%s: rendered HTML missing DOCTYPE", name)
	}
	if !strings.Contains(html, "Conductor") {
		t.Errorf("%s: rendered HTML missing brand name", name)
	}
}

func TestRenderInvite(t *testing.T) {
	const url = "https://example.com/register?token=abc123"
	html := RenderInvite(url)
	assertEmail(t, "RenderInvite", html)
	if !strings.Contains(html, url) {
		t.Errorf("RenderInvite: expected URL %q in output", url)
	}
	if !strings.Contains(html, "invited") {
		t.Errorf("RenderInvite: expected invite-specific text in output")
	}
}

func TestRenderPasswordReset(t *testing.T) {
	const url = "https://example.com/reset?token=xyz789"
	html := RenderPasswordReset(url)
	assertEmail(t, "RenderPasswordReset", html)
	if !strings.Contains(html, url) {
		t.Errorf("RenderPasswordReset: expected URL %q in output", url)
	}
	if !strings.Contains(html, "password") {
		t.Errorf("RenderPasswordReset: expected password-related text in output")
	}
}

func TestRenderAccountDisabled(t *testing.T) {
	html := RenderAccountDisabled()
	assertEmail(t, "RenderAccountDisabled", html)
	if !strings.Contains(html, "disabled") {
		t.Errorf("RenderAccountDisabled: expected 'disabled' in output")
	}
}

func TestRenderAccountEnabled(t *testing.T) {
	html := RenderAccountEnabled()
	assertEmail(t, "RenderAccountEnabled", html)
	if !strings.Contains(html, "re-enabled") {
		t.Errorf("RenderAccountEnabled: expected 're-enabled' in output")
	}
}

func TestRenderTwoFARemoved(t *testing.T) {
	html := RenderTwoFARemoved()
	assertEmail(t, "RenderTwoFARemoved", html)
	lower := strings.ToLower(html)
	if !strings.Contains(lower, "two-factor") || !strings.Contains(lower, "removed") {
		t.Errorf("RenderTwoFARemoved: expected 2FA removal text in output")
	}
}

func TestRenderTwoFAEnabled(t *testing.T) {
	html := RenderTwoFAEnabled()
	assertEmail(t, "RenderTwoFAEnabled", html)
	lower := strings.ToLower(html)
	if !strings.Contains(lower, "two-factor") || !strings.Contains(lower, "enabled") {
		t.Errorf("RenderTwoFAEnabled: expected 2FA enabled text in output")
	}
}

func TestRenderPasswordChangedByAdmin(t *testing.T) {
	html := RenderPasswordChangedByAdmin()
	assertEmail(t, "RenderPasswordChangedByAdmin", html)
	if !strings.Contains(html, "password") {
		t.Errorf("RenderPasswordChangedByAdmin: expected password text in output")
	}
}

func TestRenderPasswordChangedByReset(t *testing.T) {
	html := RenderPasswordChangedByReset()
	assertEmail(t, "RenderPasswordChangedByReset", html)
	if !strings.Contains(html, "password") {
		t.Errorf("RenderPasswordChangedByReset: expected password text in output")
	}
}

// All Render* functions must produce distinct output from each other.
func TestRenderOutputsAreDistinct(t *testing.T) {
	outputs := map[string]string{
		"invite":                 RenderInvite("https://example.com/invite"),
		"password-reset":         RenderPasswordReset("https://example.com/reset"),
		"account-disabled":       RenderAccountDisabled(),
		"account-enabled":        RenderAccountEnabled(),
		"2fa-removed":            RenderTwoFARemoved(),
		"2fa-enabled":            RenderTwoFAEnabled(),
		"password-changed-admin": RenderPasswordChangedByAdmin(),
		"password-changed-reset": RenderPasswordChangedByReset(),
	}
	seen := make(map[string]string, len(outputs))
	for name, html := range outputs {
		for prevName, prevHTML := range seen {
			if html == prevHTML {
				t.Errorf("RenderOutputsAreDistinct: %q and %q produced identical output", name, prevName)
			}
		}
		seen[name] = html
	}
}
