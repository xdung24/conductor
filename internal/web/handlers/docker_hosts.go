package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/models"
)

func (h *Handler) dockerHostStore(c *gin.Context) *models.DockerHostStore {
	return models.NewDockerHostStore(h.userDB(c))
}

// DockerHostList renders the Docker hosts management page.
func (h *Handler) DockerHostList(c *gin.Context) {
	hosts, _ := h.dockerHostStore(c).List()
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "docker_hosts.html", h.pageData(c, gin.H{
		"Hosts": hosts,
		"Flash": flash,
	}))
}

// DockerHostNew renders the create-host form.
func (h *Handler) DockerHostNew(c *gin.Context) {
	c.HTML(http.StatusOK, "docker_hosts.html", h.pageData(c, gin.H{
		"Hosts":   []*models.DockerHost{},
		"NewForm": true,
		"Host":    &models.DockerHost{},
		"Error":   "",
	}))
}

// DockerHostCreate handles the create form submission.
func (h *Handler) DockerHostCreate(c *gin.Context) {
	host, err := dockerHostFromForm(c)
	if err != nil {
		c.HTML(http.StatusBadRequest, "docker_hosts.html", h.pageData(c, gin.H{
			"Hosts":   []*models.DockerHost{},
			"NewForm": true,
			"Host":    host,
			"Error":   err.Error(),
		}))
		return
	}

	if _, err := h.dockerHostStore(c).Create(host); err != nil {
		c.HTML(http.StatusInternalServerError, "docker_hosts.html", h.pageData(c, gin.H{
			"Hosts":   []*models.DockerHost{},
			"NewForm": true,
			"Host":    host,
			"Error":   err.Error(),
		}))
		return
	}
	c.SetCookie("sm_flash", "Docker host created", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/docker-hosts")
}

// DockerHostEdit renders the edit form.
func (h *Handler) DockerHostEdit(c *gin.Context) {
	host, ok := h.getDockerHost(c)
	if !ok {
		return
	}
	hosts, _ := h.dockerHostStore(c).List()
	c.HTML(http.StatusOK, "docker_hosts.html", h.pageData(c, gin.H{
		"Hosts":    hosts,
		"EditHost": host,
		"Error":    "",
	}))
}

// DockerHostUpdate handles the edit form submission.
func (h *Handler) DockerHostUpdate(c *gin.Context) {
	existing, ok := h.getDockerHost(c)
	if !ok {
		return
	}
	updated, err := dockerHostFromForm(c)
	if err != nil {
		hosts, _ := h.dockerHostStore(c).List()
		c.HTML(http.StatusBadRequest, "docker_hosts.html", h.pageData(c, gin.H{
			"Hosts":    hosts,
			"EditHost": existing,
			"Error":    err.Error(),
		}))
		return
	}
	updated.ID = existing.ID
	if err := h.dockerHostStore(c).Update(updated); err != nil {
		hosts, _ := h.dockerHostStore(c).List()
		c.HTML(http.StatusInternalServerError, "docker_hosts.html", h.pageData(c, gin.H{
			"Hosts":    hosts,
			"EditHost": updated,
			"Error":    err.Error(),
		}))
		return
	}
	c.SetCookie("sm_flash", "Docker host updated", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/docker-hosts")
}

// DockerHostDelete removes a Docker host.
func (h *Handler) DockerHostDelete(c *gin.Context) {
	host, ok := h.getDockerHost(c)
	if !ok {
		return
	}
	if err := h.dockerHostStore(c).Delete(host.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	c.SetCookie("sm_flash", "Docker host deleted", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/docker-hosts")
}

func (h *Handler) getDockerHost(c *gin.Context) (*models.DockerHost, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "invalid docker host id"})
		return nil, false
	}
	host, err := h.dockerHostStore(c).Get(id)
	if err != nil || host == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "docker host not found"})
		return nil, false
	}
	return host, true
}

func dockerHostFromForm(c *gin.Context) (*models.DockerHost, error) {
	name := strings.TrimSpace(c.PostForm("name"))
	socketPath := strings.TrimSpace(c.PostForm("socket_path"))
	httpURL := strings.TrimSpace(c.PostForm("http_url"))

	host := &models.DockerHost{
		Name:       name,
		SocketPath: socketPath,
		HTTPURL:    httpURL,
	}

	if name == "" {
		return host, fmt.Errorf("name is required")
	}
	if socketPath == "" && httpURL == "" {
		return host, fmt.Errorf("either socket path or HTTP URL is required")
	}
	return host, nil
}
