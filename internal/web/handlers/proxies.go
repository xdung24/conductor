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

func (h *Handler) proxyStore(c *gin.Context) *models.ProxyStore {
	return models.NewProxyStore(h.userDB(c))
}

// ProxyList renders the proxy management page.
func (h *Handler) ProxyList(c *gin.Context) {
	proxies, _ := h.proxyStore(c).List()
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "proxies.html", h.pageData(c, gin.H{
		"Proxies": proxies,
		"Flash":   flash,
	}))
}

// ProxyNew renders the create-proxy form.
func (h *Handler) ProxyNew(c *gin.Context) {
	c.HTML(http.StatusOK, "proxies.html", h.pageData(c, gin.H{
		"Proxies": []*models.Proxy{},
		"NewForm": true,
		"Proxy":   &models.Proxy{},
		"Error":   "",
	}))
}

// ProxyCreate handles the create form submission.
func (h *Handler) ProxyCreate(c *gin.Context) {
	proxy, err := proxyFromForm(c)
	if err != nil {
		c.HTML(http.StatusBadRequest, "proxies.html", h.pageData(c, gin.H{
			"Proxies": []*models.Proxy{},
			"NewForm": true,
			"Proxy":   proxy,
			"Error":   err.Error(),
		}))
		return
	}

	if _, err := h.proxyStore(c).Create(proxy); err != nil {
		c.HTML(http.StatusInternalServerError, "proxies.html", h.pageData(c, gin.H{
			"Proxies": []*models.Proxy{},
			"NewForm": true,
			"Proxy":   proxy,
			"Error":   err.Error(),
		}))
		return
	}
	c.SetCookie("sm_flash", "Proxy created", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/proxies")
}

// ProxyEdit renders the edit form.
func (h *Handler) ProxyEdit(c *gin.Context) {
	proxy, ok := h.getProxy(c)
	if !ok {
		return
	}
	proxies, _ := h.proxyStore(c).List()
	c.HTML(http.StatusOK, "proxies.html", h.pageData(c, gin.H{
		"Proxies":   proxies,
		"EditProxy": proxy,
		"Error":     "",
	}))
}

// ProxyUpdate handles the edit form submission.
func (h *Handler) ProxyUpdate(c *gin.Context) {
	existing, ok := h.getProxy(c)
	if !ok {
		return
	}
	updated, err := proxyFromForm(c)
	if err != nil {
		proxies, _ := h.proxyStore(c).List()
		c.HTML(http.StatusBadRequest, "proxies.html", h.pageData(c, gin.H{
			"Proxies":   proxies,
			"EditProxy": existing,
			"Error":     err.Error(),
		}))
		return
	}
	updated.ID = existing.ID
	if err := h.proxyStore(c).Update(updated); err != nil {
		proxies, _ := h.proxyStore(c).List()
		c.HTML(http.StatusInternalServerError, "proxies.html", h.pageData(c, gin.H{
			"Proxies":   proxies,
			"EditProxy": updated,
			"Error":     err.Error(),
		}))
		return
	}
	c.SetCookie("sm_flash", "Proxy updated", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/proxies")
}

// ProxyDelete removes a proxy.
func (h *Handler) ProxyDelete(c *gin.Context) {
	proxy, ok := h.getProxy(c)
	if !ok {
		return
	}
	if err := h.proxyStore(c).Delete(proxy.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	c.SetCookie("sm_flash", "Proxy deleted", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/proxies")
}

func (h *Handler) getProxy(c *gin.Context) (*models.Proxy, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "invalid proxy id"})
		return nil, false
	}
	proxy, err := h.proxyStore(c).Get(id)
	if err != nil || proxy == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "proxy not found"})
		return nil, false
	}
	return proxy, true
}

func proxyFromForm(c *gin.Context) (*models.Proxy, error) {
	name := strings.TrimSpace(c.PostForm("name"))
	rawURL := strings.TrimSpace(c.PostForm("url"))

	proxy := &models.Proxy{
		Name: name,
		URL:  rawURL,
	}

	if name == "" {
		return proxy, fmt.Errorf("name is required")
	}
	if rawURL == "" {
		return proxy, fmt.Errorf("proxy URL is required")
	}
	if _, err := url.Parse(rawURL); err != nil {
		return proxy, fmt.Errorf("invalid proxy URL: %v", err)
	}
	return proxy, nil
}
