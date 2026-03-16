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
