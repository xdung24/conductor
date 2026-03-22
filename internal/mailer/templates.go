package mailer

import (
	"bytes"
	_ "embed"
	"html/template"
)

//go:embed template.html
var emailTemplateSource string

var emailTemplates = template.Must(template.New("").Parse(emailTemplateSource))

// emailData is the per-template data (Title and optional URL).
type emailData struct {
	Title string
	URL   string
}

// layoutData is passed to the "layout" template after the content is rendered.
type layoutData struct {
	Title string
	Body  template.HTML
}

// render executes "{name}_content" to produce the inner HTML, then wraps it
// in the shared "layout" template to produce a complete email document.
func render(name string, data emailData) string {
	// Pass 1: render the content fragment.
	var contentBuf bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&contentBuf, name+"_content", data); err != nil {
		return "email rendering error (content): " + err.Error()
	}
	// Pass 2: wrap in the layout. The content was produced by html/template so
	// it is already properly escaped; marking it as HTML.Template is safe.
	var buf bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&buf, "layout", layoutData{
		Title: data.Title,
		Body:  template.HTML(contentBuf.String()), //nolint:gosec // rendered by html/template above
	}); err != nil {
		return "email rendering error (layout): " + err.Error()
	}
	return buf.String()
}

// RenderInvite returns the HTML body for an invite email.
func RenderInvite(inviteURL string) string {
	return render("invite", emailData{Title: "You've been invited to Conductor", URL: inviteURL})
}

// RenderPasswordReset returns the HTML body for a password-reset email.
func RenderPasswordReset(resetURL string) string {
	return render("password-reset", emailData{Title: "Reset your Conductor password", URL: resetURL})
}

// RenderAccountDisabled returns the HTML body for an account-disabled email.
func RenderAccountDisabled() string {
	return render("account-disabled", emailData{Title: "Your Conductor account has been disabled"})
}

// RenderAccountEnabled returns the HTML body for an account-re-enabled email.
func RenderAccountEnabled() string {
	return render("account-enabled", emailData{Title: "Your Conductor account has been re-enabled"})
}

// RenderTwoFARemoved returns the HTML body sent when an admin removes a user's 2FA.
func RenderTwoFARemoved() string {
	return render("2fa-removed", emailData{Title: "Two-factor authentication removed"})
}

// RenderTwoFAEnabled returns the HTML body sent when the user successfully enables 2FA.
func RenderTwoFAEnabled() string {
	return render("2fa-enabled", emailData{Title: "Two-factor authentication enabled"})
}

// RenderPasswordChangedByAdmin returns the HTML body sent when an admin changes a user's password.
func RenderPasswordChangedByAdmin() string {
	return render("password-changed-admin", emailData{Title: "Your password has been changed"})
}

// RenderPasswordChangedByReset returns the HTML body sent after a successful password reset.
func RenderPasswordChangedByReset() string {
	return render("password-changed-reset", emailData{Title: "Your password has been changed"})
}
