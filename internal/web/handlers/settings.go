package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ThemeToggle handles POST /settings/theme — toggles the dark/light theme cookie.
func (h *Handler) ThemeToggle(c *gin.Context) {
	current, _ := c.Cookie("sm_theme")
	next := "dark"
	if current != "light" {
		next = "light"
	}
	// 30-day expiry; not HttpOnly so JS can read it for the inline toggle.
	c.SetCookie("sm_theme", next, 30*24*3600, "/", "", false, false)

	// Redirect back to where the user came from.
	ref := c.Request.Referer()
	if ref == "" {
		ref = "/"
	}
	c.Redirect(http.StatusFound, ref)
}

// SettingsPage renders the admin settings page.
func (h *Handler) SettingsPage(c *gin.Context) {
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "admin_settings.html", h.pageData(c, gin.H{
		"Flash":               flash,
		"Error":               "",
		"RegistrationEnabled": h.settingsStore().RegistrationEnabled(),
	}))
}

// SettingsUpdate handles POST /admin/settings — saves admin-configurable options.
func (h *Handler) SettingsUpdate(c *gin.Context) {
	regEnabled := c.PostForm("registration_enabled") == "1"
	if err := h.settingsStore().SetRegistrationEnabled(regEnabled); err != nil {
		c.HTML(http.StatusInternalServerError, "admin_settings.html", gin.H{
			"Error":               "Failed to save settings: " + err.Error(),
			"RegistrationEnabled": !regEnabled,
		})
		return
	}
	c.SetCookie("sm_flash", "Settings saved.", 60, "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin/settings")
}
