package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Home page (unauthenticated landing)
// ---------------------------------------------------------------------------

// HomePage serves the public landing page.
// If the visitor already has a valid session they are redirected to /monitors.
func (h *Handler) HomePage(c *gin.Context) {
	if token, err := c.Cookie("sm_session"); err == nil && h.validSession(token) {
		c.Redirect(http.StatusFound, "/monitors")
		return
	}
	pages, _ := h.users.ListAllStatusPageSlugs()
	c.HTML(http.StatusOK, "home.gohtml", gin.H{
		"StatusPages": pages,
	})
}
