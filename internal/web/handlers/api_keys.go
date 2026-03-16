package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/models"
)

func (h *Handler) apiKeyStore() *models.APIKeyStore {
	return models.NewAPIKeyStore(h.usersDB)
}

// APIKeyList renders the API key management page.
func (h *Handler) APIKeyList(c *gin.Context) {
	keys, _ := h.apiKeyStore().List(h.username(c))
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "api_keys.html", h.pageData(c, gin.H{
		"Keys":  keys,
		"Flash": flash,
	}))
}

// APIKeyCreate handles the create form, generates a token, and re-renders the page
// with the plain token shown once.
func (h *Handler) APIKeyCreate(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.Redirect(http.StatusFound, "/api-keys")
		return
	}

	plainToken, err := models.GenerateAPIToken()
	if err != nil {
		keys, _ := h.apiKeyStore().List(h.username(c))
		c.HTML(http.StatusInternalServerError, "api_keys.html", gin.H{
			"Keys":  keys,
			"Error": "Failed to generate token. Please try again.",
		})
		return
	}

	tokenHash := models.HashAPIToken(plainToken)
	if _, err := h.apiKeyStore().Create(h.username(c), name, tokenHash); err != nil {
		keys, _ := h.apiKeyStore().List(h.username(c))
		c.HTML(http.StatusInternalServerError, "api_keys.html", gin.H{
			"Keys":  keys,
			"Error": "Failed to save key: " + err.Error(),
		})
		return
	}

	// Re-render the list with the new plain token shown once.
	// The token is embedded directly in the HTML response — it is never stored
	// server-side again after this point.
	keys, _ := h.apiKeyStore().List(h.username(c))
	c.HTML(http.StatusOK, "api_keys.html", h.pageData(c, gin.H{
		"Keys":     keys,
		"Flash":    "API key created. Copy the token below — it won\u2019t be shown again.",
		"NewToken": plainToken,
	}))
}

// APIKeyDelete removes an API key by ID.
func (h *Handler) APIKeyDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/api-keys")
		return
	}
	_ = h.apiKeyStore().Delete(id, h.username(c))
	c.SetCookie("sm_flash", "API key deleted.", 60, "/", "", false, true)
	c.Redirect(http.StatusFound, "/api-keys")
}
