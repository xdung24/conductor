package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/models"
)

func (h *Handler) tagStore(c *gin.Context) *models.TagStore {
	return models.NewTagStore(h.userDB(c))
}

// TagList renders the tag management page.
func (h *Handler) TagList(c *gin.Context) {
	tags, _ := h.tagStore(c).List()
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "tags.html", h.pageData(c, gin.H{
		"Tags":  tags,
		"Flash": flash,
	}))
}

// TagNew renders the create-tag form.
func (h *Handler) TagNew(c *gin.Context) {
	c.HTML(http.StatusOK, "tags.html", h.pageData(c, gin.H{
		"Tags":    []*models.Tag{},
		"NewForm": true,
		"Tag":     &models.Tag{Color: "#6366f1"},
		"Error":   "",
	}))
}

// TagCreate handles tag creation form submission.
func (h *Handler) TagCreate(c *gin.Context) {
	name := c.PostForm("name")
	color := c.PostForm("color")
	if color == "" {
		color = "#6366f1"
	}

	if name == "" {
		tags, _ := h.tagStore(c).List()
		c.HTML(http.StatusBadRequest, "tags.html", gin.H{
			"Tags":    tags,
			"NewForm": true,
			"Tag":     &models.Tag{Name: name, Color: color},
			"Error":   "Tag name is required",
		})
		return
	}

	_, err := h.tagStore(c).Create(&models.Tag{Name: name, Color: color})
	if err != nil {
		tags, _ := h.tagStore(c).List()
		c.HTML(http.StatusInternalServerError, "tags.html", gin.H{
			"Tags":    tags,
			"NewForm": true,
			"Tag":     &models.Tag{Name: name, Color: color},
			"Error":   err.Error(),
		})
		return
	}

	c.SetCookie("sm_flash", "Tag created", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/tags")
}

// TagEdit renders the edit form for a specific tag.
func (h *Handler) TagEdit(c *gin.Context) {
	tag, ok := h.getTag(c)
	if !ok {
		return
	}
	tags, _ := h.tagStore(c).List()
	c.HTML(http.StatusOK, "tags.html", h.pageData(c, gin.H{
		"Tags":     tags,
		"EditForm": true,
		"Tag":      tag,
		"Error":    "",
	}))
}

// TagUpdate handles tag edit form submission.
func (h *Handler) TagUpdate(c *gin.Context) {
	tag, ok := h.getTag(c)
	if !ok {
		return
	}

	name := c.PostForm("name")
	color := c.PostForm("color")
	if color == "" {
		color = "#6366f1"
	}

	if name == "" {
		tags, _ := h.tagStore(c).List()
		c.HTML(http.StatusBadRequest, "tags.html", gin.H{
			"Tags":     tags,
			"EditForm": true,
			"Tag":      &models.Tag{ID: tag.ID, Name: name, Color: color},
			"Error":    "Tag name is required",
		})
		return
	}

	tag.Name = name
	tag.Color = color
	if err := h.tagStore(c).Update(tag); err != nil {
		tags, _ := h.tagStore(c).List()
		c.HTML(http.StatusInternalServerError, "tags.html", gin.H{
			"Tags":     tags,
			"EditForm": true,
			"Tag":      tag,
			"Error":    err.Error(),
		})
		return
	}

	c.SetCookie("sm_flash", "Tag updated", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/tags")
}

// TagDelete removes a tag.
func (h *Handler) TagDelete(c *gin.Context) {
	tag, ok := h.getTag(c)
	if !ok {
		return
	}
	if err := h.tagStore(c).Delete(tag.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	c.SetCookie("sm_flash", "Tag deleted", 5, "/", "", false, true)
	c.Redirect(http.StatusFound, "/tags")
}

// getTag is a helper that loads a tag by ID from the URL param.
func (h *Handler) getTag(c *gin.Context) (*models.Tag, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "invalid tag id"})
		return nil, false
	}
	tag, err := h.tagStore(c).Get(id)
	if err != nil || tag == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "tag not found"})
		return nil, false
	}
	return tag, true
}
