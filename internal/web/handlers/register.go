package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) regTokenStore() *models.RegistrationTokenStore {
	return models.NewRegistrationTokenStore(h.usersDB)
}

func (h *Handler) settingsStore() *models.AppSettingsStore {
	return models.NewAppSettingsStore(h.usersDB)
}

// RegisterPage renders the public self-registration form.
// Access is granted when open registration is enabled (admin setting) OR when
// a valid, unused, non-expired invite token is provided as ?token=<value>.
func (h *Handler) RegisterPage(c *gin.Context) {
	token := c.Query("token")
	if !h.registrationAllowed(token) {
		c.HTML(http.StatusForbidden, "register.html", gin.H{
			"Disabled": true,
		})
		return
	}

	c.HTML(http.StatusOK, "register.html", gin.H{
		"Token":    token,
		"Disabled": false,
		"Error":    "",
	})
}

// RegisterSubmit handles self-registration form submission.
func (h *Handler) RegisterSubmit(c *gin.Context) {
	token := c.PostForm("token")
	if !h.registrationAllowed(token) {
		c.HTML(http.StatusForbidden, "register.html", gin.H{"Disabled": true})
		return
	}

	username := c.PostForm("username")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"Token":    token,
			"Disabled": false,
			"Error":    msg,
		})
	}

	if username == "" || password == "" {
		renderErr("Username and password are required.")
		return
	}
	if len(username) < 3 || len(username) > 32 {
		renderErr("Username must be between 3 and 32 characters.")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters.")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match.")
		return
	}

	existing, _ := h.users.GetByUsername(username)
	if existing != nil {
		renderErr("That username is already taken.")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderErr("Internal error. Please try again.")
		return
	}

	if err := h.users.Create(username, string(hashed)); err != nil {
		renderErr("Failed to create account: " + err.Error())
		return
	}

	// Grant admin when registering through the startup system token.
	if token != "" {
		if rt, err := h.regTokenStore().GetPending(token); err == nil && rt != nil && rt.CreatedBy == "system" {
			_ = h.users.SetAdmin(username, true)
		}
		_ = h.regTokenStore().Consume(token)
	}

	// Initialise the new user's database and scheduler.
	db, err := h.registry.Get(username)
	if err == nil {
		h.msched.StartForUser(username, db)
	}

	// Log the user in immediately after registration.
	sessionToken := signToken(username, h.cfg.SecretKey)
	c.SetCookie(sessionCookieName, sessionToken, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

// registrationAllowed returns true when the request should be permitted to
// access the registration form. It consults the DB-backed admin setting and
// the invite token (if provided).
func (h *Handler) registrationAllowed(inviteToken string) bool {
	if h.settingsStore().RegistrationEnabled() {
		return true
	}
	if inviteToken == "" {
		return false
	}
	rt, err := h.regTokenStore().GetPending(inviteToken)
	return err == nil && rt != nil
}
