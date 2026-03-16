package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// UserList renders the user management page.
func (h *Handler) UserList(c *gin.Context) {
	users, err := h.users.ListAll()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
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
		renderErr("Username and password are required")
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

	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Password updated for "+target))
}

// UserDelete removes a user and cleans up their scheduler and DB connection.
func (h *Handler) UserDelete(c *gin.Context) {
	target := c.Param("username")
	current := h.username(c)

	if target == current {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Cannot delete your own account"))
		return
	}

	count, _ := h.users.Count()
	if count <= 1 {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Cannot delete the last user"))
		return
	}

	// Unregister push tokens, stop scheduler, close DB — then delete the record.
	_ = h.users.UnregisterAllPushTokens(target)
	h.msched.StopUser(target)
	h.registry.Remove(target)

	if err := h.users.Delete(target); err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Failed to delete user: "+err.Error()))
		return
	}

	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("User "+target+" deleted"))
}

// InviteGenerate creates a single-use registration token and redirects to the
// users page with the full invite URL shown as a flash message.
func (h *Handler) InviteGenerate(c *gin.Context) {
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
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Invite link: "+inviteURL))
}

// InviteRevoke deletes an invite token.
func (h *Handler) InviteRevoke(c *gin.Context) {
	token := c.Param("token")
	_ = h.regTokenStore().Delete(token)
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape("Invite revoked"))
}

// UserSetAdmin grants or revokes admin privileges for a user.
func (h *Handler) UserSetAdmin(c *gin.Context) {
	target := c.Param("username")
	current := h.username(c)

	if target == current {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Cannot change your own admin role"))
		return
	}

	u, _ := h.users.GetByUsername(target)
	if u == nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("User not found"))
		return
	}

	// Toggle: if currently admin, revoke; otherwise grant.
	grant := !u.IsAdmin
	if err := h.users.SetAdmin(target, grant); err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error="+url.QueryEscape("Failed to update role: "+err.Error()))
		return
	}

	var msg string
	if grant {
		msg = target + " is now an admin"
	} else {
		msg = target + " is no longer an admin"
	}
	c.Redirect(http.StatusFound, "/admin/users?flash="+url.QueryEscape(msg))
}
