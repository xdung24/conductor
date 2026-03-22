package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/mailer"
	"github.com/xdung24/conductor/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) pwResetStore() *models.PasswordResetTokenStore {
	return models.NewPasswordResetTokenStore(h.usersDB)
}

const userPageSize = 10

// UserList renders the user management page with search + pagination.
func (h *Handler) UserList(c *gin.Context) {
	q := c.Query("q")
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}

	users, total, err := h.users.ListPaged(q, page, userPageSize)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	totalPages := int(math.Ceil(float64(total) / float64(userPageSize)))
	if totalPages < 1 {
		totalPages = 1
	}

	inviteTokens, _ := h.regTokenStore().ListAll()
	c.HTML(http.StatusOK, "users.html", gin.H{
		"Users":               users,
		"CurrentUser":         h.username(c),
		"IsAdmin":             h.isAdmin(c),
		"Flash":               c.Query("flash"),
		"FlashError":          c.Query("error"),
		"InviteTokens":        inviteTokens,
		"RegistrationEnabled": h.settingsStore().RegistrationEnabled(),
		"Q":                   q,
		"Page":                page,
		"TotalPages":          totalPages,
		"Total":               total,
	})
}

// UserNew renders the create-user form.
func (h *Handler) UserNew(c *gin.Context) {
	c.HTML(http.StatusOK, "user_form.html", gin.H{
		"IsNew":      true,
		"IsAdmin":    h.isAdmin(c),
		"TargetUser": "",
		"Username":   "",
		"Error":      "",
	})
}

// UserCreate handles new user form submission.
func (h *Handler) UserCreate(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "user_form.html", gin.H{
			"IsNew": true, "TargetUser": "", "Username": username, "Error": msg,
		})
	}

	if username == "" || password == "" {
		renderErr("Email and password are required")
		return
	}
	canonical, emailErr := validateEmail(username)
	if emailErr != nil {
		renderErr(emailErr.Error())
		return
	}
	username = canonical
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match")
		return
	}

	existing, _ := h.users.GetByUsername(username)
	if existing != nil {
		renderErr("Username already exists")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "user_form.html", gin.H{
			"IsNew": true, "TargetUser": "", "Username": username, "Error": "Internal error",
		})
		return
	}

	if err := h.users.Create(username, string(hashed)); err != nil {
		c.HTML(http.StatusInternalServerError, "user_form.html", gin.H{
			"IsNew": true, "TargetUser": "", "Username": username,
			"Error": "Failed to create user: " + err.Error(),
		})
		return
	}

	// Initialise the new user's database and scheduler.
	db, err := h.registry.Get(username)
	if err == nil {
		h.msched.StartForUser(username, db)
	}

	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("User "+username+" created"))
}

// UserPasswordPage renders the change-password form for a user.
func (h *Handler) UserPasswordPage(c *gin.Context) {
	target := c.Param("username")
	u, _ := h.users.GetByUsername(target)
	if u == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "User not found"})
		return
	}
	c.HTML(http.StatusOK, "user_form.html", gin.H{
		"IsNew":      false,
		"IsAdmin":    h.isAdmin(c),
		"TargetUser": target,
		"Error":      "",
	})
}

// UserChangePassword handles the change-password form submission.
func (h *Handler) UserChangePassword(c *gin.Context) {
	target := c.Param("username")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "user_form.html", gin.H{
			"IsNew": false, "TargetUser": target, "Error": msg,
		})
	}

	if password == "" {
		renderErr("Password is required")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match")
		return
	}

	u, _ := h.users.GetByUsername(target)
	if u == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "User not found"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderErr("Internal error")
		return
	}

	if err := h.users.UpdatePassword(target, string(hashed)); err != nil {
		renderErr("Failed to update password: " + err.Error())
		return
	}

	h.mailer.SendAsync(target, "Your password has been changed", mailer.RenderPasswordChangedByAdmin())
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Password updated for "+target))
}

// InviteGenerate creates a single-use registration token and redirects to the
// users page with the full invite URL shown as a flash message.
// The recipient email is mandatory — it is used address the email (when the
// mailer is configured) and displayed in the flash for copy-paste fallback.
func (h *Handler) InviteGenerate(c *gin.Context) {
	recipientEmail := c.PostForm("recipient_email")
	if recipientEmail == "" {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Recipient email is required"))
		return
	}
	if _, err := validateEmail(recipientEmail); err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Invalid recipient email: "+err.Error()))
		return
	}

	token, err := h.regTokenStore().Generate(h.username(c), 0) // 0 = no expiry for admin invites
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Failed to generate invite: "+err.Error()))
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	inviteURL := fmt.Sprintf("%s://%s/register?token=%s", scheme, c.Request.Host, token)

	// Send invite email (fire-and-forget, no-op when SMTP is unconfigured).
	h.mailer.SendAsync(recipientEmail, "You've been invited to Conductor", mailer.RenderInvite(inviteURL))

	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Invite link for "+recipientEmail+" (copy and send manually): "+inviteURL))
}

// InviteRevoke deletes an invite token.
func (h *Handler) InviteRevoke(c *gin.Context) {
	token := c.Param("token")
	_ = h.regTokenStore().Delete(token)
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Invite revoked"))
}

// UserGenerateResetLink creates a 30-minute password-reset token and redirects
// to the admin users page with the full reset URL in the flash message.
func (h *Handler) UserGenerateResetLink(c *gin.Context) {
	target := c.Param("username")
	u, _ := h.users.GetByUsername(target)
	if u == nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("User not found"))
		return
	}

	token, err := h.pwResetStore().Generate(target)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Failed to generate reset link: "+err.Error()))
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	resetURL := fmt.Sprintf("%s://%s/reset-password?token=%s", scheme, c.Request.Host, token)

	// Send password-reset email (fire-and-forget).
	h.mailer.SendAsync(target, "Reset your Conductor password", mailer.RenderPasswordReset(resetURL))

	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Password reset link for "+target+" (valid 30 min): "+resetURL))
}

// UserToggleDisabled enables or disables a user account.
// A disabled account is immediately locked out of all sessions and its monitor
// scheduler is stopped. Re-enabling restarts the scheduler.
func (h *Handler) UserToggleDisabled(c *gin.Context) {
	target := c.Param("username")
	current := h.username(c)

	if target == current {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Cannot disable your own account"))
		return
	}

	u, _ := h.users.GetByUsername(target)
	if u == nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("User not found"))
		return
	}

	// Prevent disabling the last admin.
	if u.IsAdmin && !u.Disabled {
		adminCount, err := h.users.CountAdmins()
		if err == nil && adminCount <= 1 {
			c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Cannot disable the last admin account"))
			return
		}
	}

	nowDisabled := !u.Disabled
	if err := h.users.SetDisabled(target, nowDisabled); err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Failed to update account: "+err.Error()))
		return
	}

	if nowDisabled {
		// Stop all background monitor jobs for the disabled user.
		h.msched.StopUser(target)
	} else {
		// Restart the scheduler when re-enabling.
		db, err := h.registry.Get(target)
		if err == nil {
			h.msched.StartForUser(target, db)
		}
	}

	var msg string
	if nowDisabled {
		msg = target + " has been disabled"
		h.mailer.SendAsync(target, "Your Conductor account has been disabled", mailer.RenderAccountDisabled())
	} else {
		msg = target + " has been enabled"
		h.mailer.SendAsync(target, "Your Conductor account has been re-enabled", mailer.RenderAccountEnabled())
	}
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape(msg))
}

// ResetPasswordPage renders the public password-reset form.
// Access is granted only with a valid, unexpired, unused token.
func (h *Handler) ResetPasswordPage(c *gin.Context) {
	token := c.Query("token")
	rt, err := h.pwResetStore().GetValid(token)
	if err != nil || rt == nil {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"Invalid": true,
			"Error":   "",
		})
		return
	}
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"Invalid":  false,
		"Token":    token,
		"Username": rt.Username,
		"Error":    "",
	})
}

// ResetPasswordSubmit handles the password-reset form submission.
func (h *Handler) ResetPasswordSubmit(c *gin.Context) {
	token := c.PostForm("token")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "reset_password.html", gin.H{
			"Invalid": false,
			"Token":   token,
			"Error":   msg,
		})
	}

	rt, err := h.pwResetStore().GetValid(token)
	if err != nil || rt == nil {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{"Invalid": true, "Error": ""})
		return
	}

	if password == "" {
		renderErr("Password is required")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderErr("Internal error")
		return
	}

	if err := h.users.UpdatePassword(rt.Username, string(hashed)); err != nil {
		renderErr("Failed to update password: " + err.Error())
		return
	}

	// Consume the token so it cannot be reused.
	_ = h.pwResetStore().Consume(token)

	// Notify the user that their password was changed via the reset link.
	h.mailer.SendAsync(rt.Username, "Your password has been changed", mailer.RenderPasswordChangedByReset())

	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"Invalid": false,
		"Done":    true,
		"Error":   "",
	})
}

// UserRemove2FA clears the TOTP secret for a user (lost-device recovery).
// Only admins can call this; they cannot remove their own 2FA via this endpoint.
func (h *Handler) UserRemove2FA(c *gin.Context) {
	target := c.Param("username")
	current := h.username(c)

	if target == current {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Use /account/2fa to manage your own 2FA"))
		return
	}

	u, _ := h.users.GetByUsername(target)
	if u == nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("User not found"))
		return
	}

	if err := h.users.DisableTOTP(target); err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Failed to remove 2FA: "+err.Error()))
		return
	}

	h.mailer.SendAsync(target, "Two-factor authentication removed", mailer.RenderTwoFARemoved())
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("2FA removed for "+target))
}
