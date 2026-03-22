package mailer

import "fmt"

// ---------------------------------------------------------------------------
// Shared HTML email layout
// ---------------------------------------------------------------------------

func wrap(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>%s</title>
</head>
<body style="margin:0;padding:0;background:#f8fafc;font-family:sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" style="background:#f8fafc;padding:40px 20px;">
<tr><td align="center">
<table width="520" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;border:1px solid #e2e8f0;">
  <tr><td style="background:#0f172a;padding:24px 32px;">
    <span style="font-size:1.25rem;font-weight:700;color:#f1f5f9;">&#9829; <span style="color:#38bdf8;">Conductor</span></span>
  </td></tr>
  <tr><td style="padding:32px;">
    <p style="font-size:1rem;font-weight:600;color:#1e293b;margin:0 0 16px;">%s</p>
    %s
    <p style="font-size:0.75rem;color:#94a3b8;margin:32px 0 0;border-top:1px solid #e2e8f0;padding-top:16px;">
      This is an automated message from Conductor. Do not reply to this email.
    </p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>`, title, title, body)
}

func p(text string) string {
	return fmt.Sprintf(`<p style="font-size:0.9rem;color:#334155;margin:0 0 12px;">%s</p>`, text)
}

func btn(href, label string) string {
	return fmt.Sprintf(`<p style="margin:24px 0;">
  <a href="%s" style="background:#38bdf8;color:#0f172a;text-decoration:none;padding:10px 22px;border-radius:6px;font-weight:600;font-size:0.9rem;display:inline-block;">%s</a>
</p>`, href, label)
}

func note(text string) string {
	return fmt.Sprintf(`<p style="font-size:0.8rem;color:#64748b;margin:8px 0;">%s</p>`, text)
}

// ---------------------------------------------------------------------------
// Email render functions
// ---------------------------------------------------------------------------

// RenderInvite returns the HTML body for an invite email.
func RenderInvite(inviteURL string) string {
	body := p("You've been invited to join Conductor, a self-hosted uptime monitoring platform.") +
		p("Click the button below to create your account. The link is single-use.") +
		btn(inviteURL, "Create Account") +
		note("If you weren't expecting this invitation, you can safely ignore this email.") +
		note("Link: "+inviteURL)
	return wrap("You've been invited to Conductor", body)
}

// RenderPasswordReset returns the HTML body for a password-reset email.
func RenderPasswordReset(resetURL string) string {
	body := p("An administrator has generated a password reset link for your account.") +
		p("Click the button below to set a new password. The link expires in <strong>30 minutes</strong> and can only be used once.") +
		btn(resetURL, "Reset Password") +
		note("If you did not request this, please contact your administrator.") +
		note("Link: "+resetURL)
	return wrap("Reset your Conductor password", body)
}

// RenderAccountDisabled returns the HTML body for an account-disabled email.
func RenderAccountDisabled() string {
	body := p("Your Conductor account has been <strong>disabled</strong> by an administrator.") +
		p("You will not be able to log in or access your monitors until the account is re-enabled.") +
		p("If you believe this is an error, please contact your administrator.")
	return wrap("Your Conductor account has been disabled", body)
}

// RenderAccountEnabled returns the HTML body for an account-re-enabled email.
func RenderAccountEnabled() string {
	body := p("Your Conductor account has been <strong>re-enabled</strong>.") +
		p("You can now log in and access your monitors as usual.")
	return wrap("Your Conductor account has been re-enabled", body)
}

// RenderTwoFARemoved returns the HTML body sent when an admin removes a user's 2FA.
func RenderTwoFARemoved() string {
	body := p("An administrator has removed two-factor authentication (2FA) from your Conductor account.") +
		p("If you did not request this, please contact your administrator immediately and consider changing your password.")
	return wrap("Two-factor authentication removed", body)
}

// RenderTwoFAEnabled returns the HTML body sent when the user successfully enables 2FA.
func RenderTwoFAEnabled() string {
	body := p("Two-factor authentication (2FA) has been successfully enabled on your Conductor account.") +
		p("Each login will now require a one-time code from your authenticator app.")
	return wrap("Two-factor authentication enabled", body)
}

// RenderPasswordChangedByAdmin returns the HTML body sent when an admin changes a user's password.
func RenderPasswordChangedByAdmin() string {
	body := p("An administrator has changed the password for your Conductor account.") +
		p("If you did not request this change, please contact your administrator immediately.")
	return wrap("Your password has been changed", body)
}

// RenderPasswordChangedByReset returns the HTML body sent after a successful password reset.
func RenderPasswordChangedByReset() string {
	body := p("Your Conductor password has been successfully changed via the password reset link.") +
		p("If you did not make this change, please contact your administrator immediately.")
	return wrap("Your password has been changed", body)
}
