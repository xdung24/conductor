package handlers

import (
	"encoding/json"
	"net/http"
	neturl "net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/models"
)

// MonitorNew renders the new monitor form.
func (h *Handler) MonitorNew(c *gin.Context) {
	allNotifs, _ := h.notifications.List()
	c.HTML(http.StatusOK, "monitor_form.html", gin.H{
		"Monitor":        &models.Monitor{IntervalSeconds: 60, TimeoutSeconds: 30, Retries: 1},
		"IsNew":          true,
		"Error":          "",
		"AllNotifs":      allNotifs,
		"LinkedNotifIDs": map[int64]bool{},
		"NotifSummaries": notifSummaryMap(allNotifs),
	})
}

// MonitorCreate handles new monitor form submission.
func (h *Handler) MonitorCreate(c *gin.Context) {
	m, err := monitorFromForm(c)
	if err != nil {
		allNotifs, _ := h.notifications.List()
		c.HTML(http.StatusBadRequest, "monitor_form.html", gin.H{
			"Monitor": m, "IsNew": true, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": map[int64]bool{},
			"NotifSummaries": notifSummaryMap(allNotifs),
		})
		return
	}

	id, err := h.monitors.Create(m)
	if err != nil {
		allNotifs, _ := h.notifications.List()
		c.HTML(http.StatusInternalServerError, "monitor_form.html", gin.H{
			"Monitor": m, "IsNew": true, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": map[int64]bool{},
			"NotifSummaries": notifSummaryMap(allNotifs),
		})
		return
	}

	m.ID = id
	_ = h.notifications.ReplaceMonitorLinks(m.ID, notifIDsFromForm(c))
	h.sched.Schedule(m)
	c.Redirect(http.StatusFound, "/")
}

// MonitorDetail renders a monitor's heartbeat history.
func (h *Handler) MonitorDetail(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}

	beats, _ := h.heartbeat.Latest(m.ID, 100)
	uptime24h, _ := h.heartbeat.UptimePercent(m.ID, time.Now().Add(-24*time.Hour))
	uptime30d, _ := h.heartbeat.UptimePercent(m.ID, time.Now().Add(-30*24*time.Hour))

	c.HTML(http.StatusOK, "monitor_detail.html", gin.H{
		"Monitor":   m,
		"Beats":     beats,
		"Uptime24h": uptime24h,
		"Uptime30d": uptime30d,
	})
}

// MonitorEdit renders the edit form for an existing monitor.
func (h *Handler) MonitorEdit(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	allNotifs, _ := h.notifications.List()
	linked, _ := h.notifications.ListForMonitor(m.ID)
	linkedIDs := make(map[int64]bool, len(linked))
	for _, n := range linked {
		linkedIDs[n.ID] = true
	}
	c.HTML(http.StatusOK, "monitor_form.html", gin.H{
		"Monitor":        m,
		"IsNew":          false,
		"Error":          "",
		"AllNotifs":      allNotifs,
		"LinkedNotifIDs": linkedIDs,
		"NotifSummaries": notifSummaryMap(allNotifs),
	})
}

// MonitorUpdate handles the edit form submission.
func (h *Handler) MonitorUpdate(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}

	updated, err := monitorFromForm(c)
	if err != nil {
		allNotifs, _ := h.notifications.List()
		linked, _ := h.notifications.ListForMonitor(m.ID)
		linkedIDs := make(map[int64]bool, len(linked))
		for _, n := range linked {
			linkedIDs[n.ID] = true
		}
		c.HTML(http.StatusBadRequest, "monitor_form.html", gin.H{
			"Monitor": m, "IsNew": false, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": linkedIDs,
			"NotifSummaries": notifSummaryMap(allNotifs),
		})
		return
	}
	updated.ID = m.ID

	if err := h.monitors.Update(updated); err != nil {
		allNotifs, _ := h.notifications.List()
		c.HTML(http.StatusInternalServerError, "monitor_form.html", gin.H{
			"Monitor": updated, "IsNew": false, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": map[int64]bool{},
			"NotifSummaries": notifSummaryMap(allNotifs),
		})
		return
	}

	_ = h.notifications.ReplaceMonitorLinks(updated.ID, notifIDsFromForm(c))
	h.sched.Schedule(updated)
	c.Redirect(http.StatusFound, "/")
}

// MonitorDelete removes a monitor.
func (h *Handler) MonitorDelete(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	h.sched.Unschedule(m.ID)
	h.monitors.Delete(m.ID)
	c.Redirect(http.StatusFound, "/")
}

// MonitorPause pauses a monitor.
func (h *Handler) MonitorPause(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	h.monitors.SetActive(m.ID, false)
	h.sched.Unschedule(m.ID)
	c.Redirect(http.StatusFound, "/")
}

// MonitorResume resumes a paused monitor.
func (h *Handler) MonitorResume(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	h.monitors.SetActive(m.ID, true)
	m.Active = true
	h.sched.Schedule(m)
	c.Redirect(http.StatusFound, "/")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *Handler) getMonitor(c *gin.Context) (*models.Monitor, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "Invalid monitor ID"})
		return nil, false
	}

	m, err := h.monitors.Get(id)
	if err != nil || m == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "Monitor not found"})
		return nil, false
	}
	return m, true
}

func monitorFromForm(c *gin.Context) (*models.Monitor, error) {
	intervalSec, err := strconv.Atoi(c.DefaultPostForm("interval_seconds", "60"))
	if err != nil || intervalSec < 20 {
		intervalSec = 60
	}
	timeoutSec, err := strconv.Atoi(c.DefaultPostForm("timeout_seconds", "30"))
	if err != nil || timeoutSec < 1 {
		timeoutSec = 30
	}
	retries, err := strconv.Atoi(c.DefaultPostForm("retries", "1"))
	if err != nil || retries < 0 {
		retries = 1
	}

	name := c.PostForm("name")
	monURL := c.PostForm("url")
	monType := models.MonitorType(c.DefaultPostForm("type", "http"))
	dnsServer := c.PostForm("dns_server")

	// Always build a partial monitor so error paths never get nil.
	m := &models.Monitor{
		Name:            name,
		Type:            monType,
		URL:             monURL,
		IntervalSeconds: intervalSec,
		TimeoutSeconds:  timeoutSec,
		Active:          true,
		Retries:         retries,
		DNSServer:       dnsServer,
	}
	if name == "" {
		return m, &formError{"name is required"}
	}
	if monURL == "" {
		return m, &formError{"url is required"}
	}
	return m, nil
}

type formError struct{ msg string }

func (e *formError) Error() string { return e.msg }

// notifIDsFromForm parses the repeated "notifications" form values into a slice of int64 IDs.
func notifIDsFromForm(c *gin.Context) []int64 {
	vals := c.PostFormArray("notifications")
	ids := make([]int64, 0, len(vals))
	for _, v := range vals {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// notifSummaryMap returns a map of notification ID → human-readable config summary
// (non-sensitive) for display in the monitor form.
func notifSummaryMap(notifs []*models.Notification) map[int64]string {
	summaries := make(map[int64]string, len(notifs))
	for _, n := range notifs {
		var cfg map[string]string
		_ = json.Unmarshal([]byte(n.Config), &cfg)
		switch n.Type {
		case "webhook":
			if u := cfg["url"]; u != "" {
				if parsed, err := neturl.Parse(u); err == nil && parsed.Host != "" {
					summaries[n.ID] = parsed.Host
				} else {
					summaries[n.ID] = u
				}
			}
		case "telegram":
			if id := cfg["chat_id"]; id != "" {
				summaries[n.ID] = "Chat: " + id
			}
		case "email":
			if to := cfg["to"]; to != "" {
				summaries[n.ID] = "→ " + to
			}
		}
	}
	return summaries
}
