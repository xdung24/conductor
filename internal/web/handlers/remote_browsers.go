package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/models"
)

func (h *Handler) remoteBrowserStore(c *gin.Context) *models.RemoteBrowserStore {
	return models.NewRemoteBrowserStore(h.userDB(c))
}

// RemoteBrowserList renders the remote browser management page.
func (h *Handler) RemoteBrowserList(c *gin.Context) {
	items, _ := h.remoteBrowserStore(c).List()
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "remote_browsers.gohtml", h.pageData(c, gin.H{
		"Browsers": items,
		"Flash":    flash,
	}))
}

// RemoteBrowserNew renders the create form.
func (h *Handler) RemoteBrowserNew(c *gin.Context) {
	c.HTML(http.StatusOK, "remote_browsers.gohtml", h.pageData(c, gin.H{
		"Browsers": []*models.RemoteBrowser{},
		"NewForm":  true,
		"Browser":  &models.RemoteBrowser{},
		"Error":    "",
	}))
}

// RemoteBrowserCreate handles create submission.
func (h *Handler) RemoteBrowserCreate(c *gin.Context) {
	rb, err := remoteBrowserFromForm(c)
	if err != nil {
		c.HTML(http.StatusBadRequest, "remote_browsers.gohtml", h.pageData(c, gin.H{
			"Browsers": []*models.RemoteBrowser{},
			"NewForm":  true,
			"Browser":  rb,
			"Error":    err.Error(),
		}))
		return
	}

	if _, err := h.remoteBrowserStore(c).Create(rb); err != nil {
		c.HTML(http.StatusInternalServerError, "remote_browsers.gohtml", h.pageData(c, gin.H{
			"Browsers": []*models.RemoteBrowser{},
			"NewForm":  true,
			"Browser":  rb,
			"Error":    err.Error(),
		}))
		return
	}
	c.SetCookie("sm_flash", "Remote browser created", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/remote-browsers")
}

// RemoteBrowserEdit renders edit form.
func (h *Handler) RemoteBrowserEdit(c *gin.Context) {
	rb, ok := h.getRemoteBrowser(c)
	if !ok {
		return
	}
	items, _ := h.remoteBrowserStore(c).List()
	c.HTML(http.StatusOK, "remote_browsers.gohtml", h.pageData(c, gin.H{
		"Browsers":    items,
		"EditBrowser": rb,
		"Error":       "",
	}))
}

// RemoteBrowserUpdate handles edit submission.
func (h *Handler) RemoteBrowserUpdate(c *gin.Context) {
	existing, ok := h.getRemoteBrowser(c)
	if !ok {
		return
	}
	rb, err := remoteBrowserFromForm(c)
	if err != nil {
		items, _ := h.remoteBrowserStore(c).List()
		c.HTML(http.StatusBadRequest, "remote_browsers.gohtml", h.pageData(c, gin.H{
			"Browsers":    items,
			"EditBrowser": existing,
			"Error":       err.Error(),
		}))
		return
	}
	rb.ID = existing.ID

	if err := h.remoteBrowserStore(c).Update(rb); err != nil {
		items, _ := h.remoteBrowserStore(c).List()
		c.HTML(http.StatusInternalServerError, "remote_browsers.gohtml", h.pageData(c, gin.H{
			"Browsers":    items,
			"EditBrowser": rb,
			"Error":       err.Error(),
		}))
		return
	}
	c.SetCookie("sm_flash", "Remote browser updated", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/remote-browsers")
}

// RemoteBrowserDelete removes an item.
func (h *Handler) RemoteBrowserDelete(c *gin.Context) {
	rb, ok := h.getRemoteBrowser(c)
	if !ok {
		return
	}
	if err := h.remoteBrowserStore(c).Delete(rb.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.gohtml", gin.H{"Error": err.Error()})
		return
	}
	c.SetCookie("sm_flash", "Remote browser deleted", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/remote-browsers")
}

func (h *Handler) getRemoteBrowser(c *gin.Context) (*models.RemoteBrowser, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.gohtml", gin.H{"Error": "invalid remote browser id"})
		return nil, false
	}
	rb, err := h.remoteBrowserStore(c).Get(id)
	if err != nil || rb == nil {
		c.HTML(http.StatusNotFound, "error.gohtml", gin.H{"Error": "remote browser not found"})
		return nil, false
	}
	return rb, true
}

func remoteBrowserFromForm(c *gin.Context) (*models.RemoteBrowser, error) {
	name := strings.TrimSpace(c.PostForm("name"))
	endpoint := strings.TrimSpace(c.PostForm("endpoint_url"))
	rb := &models.RemoteBrowser{Name: name, EndpointURL: endpoint}

	if name == "" {
		return rb, fmt.Errorf("name is required")
	}
	if endpoint == "" {
		return rb, fmt.Errorf("endpoint URL is required")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return rb, fmt.Errorf("invalid endpoint URL: %v", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return rb, fmt.Errorf("endpoint URL must use ws:// or wss://")
	}
	return rb, nil
}
