package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/models"
	"github.com/xdung24/service-monitor/internal/notifier"
)

// NotificationList renders the notifications management page.
func (h *Handler) NotificationList(c *gin.Context) {
	nstore := h.notifStore(c)
	notifs, err := nstore.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}

	// Surface flash messages set by Test/Delete redirects.
	testedName := ""
	if idStr := c.Query("tested"); idStr != "" {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			if n, _ := nstore.Get(id); n != nil {
				testedName = n.Name
			}
		}
	}

	c.HTML(http.StatusOK, "notification_list.html", h.pageData(c, gin.H{
		"Notifications": notifs,
		"Tested":        testedName,
		"FlashError":    c.Query("error"),
	}))
}

// NotificationNew renders the new notification form.
func (h *Handler) NotificationNew(c *gin.Context) {
	c.HTML(http.StatusOK, "notification_form.html", h.pageData(c, gin.H{
		"Notification": &models.Notification{Active: true},
		"IsNew":        true,
		"Error":        "",
		"Config":       map[string]string{},
	}))
}

// NotificationCreate handles new notification form submission.
func (h *Handler) NotificationCreate(c *gin.Context) {
	n, cfgJSON, err := notificationFromForm(c)
	if err != nil {
		c.HTML(http.StatusBadRequest, "notification_form.html", gin.H{
			"Notification": n, "IsNew": true, "Error": err.Error(),
			"Config": notificationConfigMap(cfgJSON),
		})
		return
	}
	n.Config = cfgJSON

	id, err := h.notifStore(c).Create(n)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "notification_form.html", gin.H{
			"Notification": n, "IsNew": true, "Error": err.Error(),
			"Config": notificationConfigMap(cfgJSON),
		})
		return
	}
	_ = id
	c.Redirect(http.StatusFound, "/notifications")
}

// NotificationEdit renders the edit form for an existing notification.
func (h *Handler) NotificationEdit(c *gin.Context) {
	n, ok := h.getNotification(c)
	if !ok {
		return
	}
	c.HTML(http.StatusOK, "notification_form.html", h.pageData(c, gin.H{
		"Notification": n,
		"IsNew":        false,
		"Error":        "",
		"Config":       notificationConfigMap(n.Config),
	}))
}

// NotificationUpdate handles the edit form submission.
func (h *Handler) NotificationUpdate(c *gin.Context) {
	existing, ok := h.getNotification(c)
	if !ok {
		return
	}

	n, cfgJSON, err := notificationFromForm(c)
	if err != nil {
		c.HTML(http.StatusBadRequest, "notification_form.html", gin.H{
			"Notification": existing, "IsNew": false, "Error": err.Error(),
			"Config": notificationConfigMap(cfgJSON),
		})
		return
	}
	n.ID = existing.ID
	n.Config = cfgJSON

	if err := h.notifStore(c).Update(n); err != nil {
		c.HTML(http.StatusInternalServerError, "notification_form.html", gin.H{
			"Notification": n, "IsNew": false, "Error": err.Error(),
			"Config": notificationConfigMap(cfgJSON),
		})
		return
	}
	c.Redirect(http.StatusFound, "/notifications")
}

// NotificationDelete removes a notification provider.
func (h *Handler) NotificationDelete(c *gin.Context) {
	n, ok := h.getNotification(c)
	if !ok {
		return
	}
	h.notifStore(c).Delete(n.ID)
	c.Redirect(http.StatusFound, "/notifications")
}

// NotificationTest sends a test event for a notification provider.
func (h *Handler) NotificationTest(c *gin.Context) {
	n, ok := h.getNotification(c)
	if !ok {
		return
	}

	var cfg map[string]string
	if err := json.Unmarshal([]byte(n.Config), &cfg); err != nil {
		c.Redirect(http.StatusFound, "/notifications?error=invalid+config+JSON")
		return
	}

	p, exists := notifier.Registry[n.Type]
	if !exists {
		c.Redirect(http.StatusFound, "/notifications?error=unknown+provider+type")
		return
	}

	testEvent := notifier.Event{
		MonitorID:   0,
		MonitorName: "[Test]",
		MonitorURL:  "https://example.com",
		Status:      1,
		LatencyMs:   42,
		Message:     "This is a test notification from Service Monitor.",
	}

	sendErr := p.Send(c.Request.Context(), cfg, testEvent)

	// Log the test attempt.
	errStr := ""
	if sendErr != nil {
		errStr = sendErr.Error()
	}
	_ = h.notifLogStore(c).Insert(&models.NotificationLog{
		NotificationID:   &n.ID,
		MonitorName:      "[Test]",
		NotificationName: n.Name,
		EventStatus:      1,
		Success:          sendErr == nil,
		Error:            errStr,
		CreatedAt:        time.Now().UTC(),
	})

	if sendErr != nil {
		c.Redirect(http.StatusFound, "/notifications?error="+url.QueryEscape(sendErr.Error()))
		return
	}
	c.Redirect(http.StatusFound, "/notifications?tested="+c.Param("id"))
}

// NotificationLogList renders the notification send history page.
func (h *Handler) NotificationLogList(c *gin.Context) {
	logs, err := h.notifLogStore(c).List(200)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	c.HTML(http.StatusOK, "notification_log.html", h.pageData(c, gin.H{"Logs": logs}))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *Handler) getNotification(c *gin.Context) (*models.Notification, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "Invalid notification ID"})
		return nil, false
	}
	n, err := h.notifStore(c).Get(id)
	if err != nil || n == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "Notification not found"})
		return nil, false
	}
	return n, true
}

// notificationFromForm parses the form, builds a Notification and returns the
// JSON-encoded config string. Always returns a non-nil Notification so error
// paths in handlers can safely render the form without nil-pointer panics.
func notificationFromForm(c *gin.Context) (*models.Notification, string, error) {
	name := c.PostForm("name")
	ntype := c.PostForm("type")
	activeStr := c.PostForm("active")

	// Build config map from type-specific fields first (even when empty/invalid).
	cfg := make(map[string]string)
	switch ntype {
	case "webhook":
		cfg["url"] = c.PostForm("cfg_url")
		cfg["secret"] = c.PostForm("cfg_secret")
	case "telegram":
		cfg["bot_token"] = c.PostForm("cfg_bot_token")
		cfg["chat_id"] = c.PostForm("cfg_chat_id")
	case "email":
		cfg["host"] = c.PostForm("cfg_host")
		cfg["port"] = c.PostForm("cfg_port")
		cfg["username"] = c.PostForm("cfg_username")
		cfg["password"] = c.PostForm("cfg_password")
		cfg["from"] = c.PostForm("cfg_from")
		cfg["to"] = c.PostForm("cfg_to")
		cfg["tls"] = c.DefaultPostForm("cfg_tls", "true")
	case "slack":
		cfg["url"] = c.PostForm("cfg_slack_url")
	case "discord":
		cfg["url"] = c.PostForm("cfg_discord_url")
	case "ntfy":
		cfg["topic"] = c.PostForm("cfg_ntfy_topic")
		cfg["server"] = c.PostForm("cfg_ntfy_server")
		cfg["token"] = c.PostForm("cfg_ntfy_token")
	}
	cfgBytes, _ := json.Marshal(cfg)
	cfgJSON := string(cfgBytes)

	// Build the notification struct before validation so callers always get a
	// non-nil value they can pass back to the template.
	n := &models.Notification{
		Name:   name,
		Type:   ntype,
		Active: activeStr == "on" || activeStr == "true" || activeStr == "1",
		Config: cfgJSON,
	}
	if name == "" {
		return n, cfgJSON, &formError{"name is required"}
	}
	if ntype == "" {
		return n, cfgJSON, &formError{"type is required"}
	}
	return n, cfgJSON, nil
}

// notificationConfigMap decodes the JSON config blob into a map for template rendering.
func notificationConfigMap(configJSON string) map[string]string {
	m := make(map[string]string)
	_ = json.Unmarshal([]byte(configJSON), &m)
	return m
}
