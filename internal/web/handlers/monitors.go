package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
		"Monitor":        &models.Monitor{IntervalSeconds: 60, TimeoutSeconds: 30, Retries: 1, NotifyOnFailure: true, NotifyOnSuccess: true},
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

// MonitorExport streams a single monitor's config as a downloadable JSON file.
// The exported file contains only user-editable fields (no ID, no timestamps,
// no runtime state) plus a schema version for forward-compatibility.
func (h *Handler) MonitorExport(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}

	type exportDoc struct {
		Schema               string             `json:"schema"`
		Name                 string             `json:"name"`
		Type                 models.MonitorType `json:"type"`
		URL                  string             `json:"url"`
		IntervalSeconds      int                `json:"interval_seconds"`
		TimeoutSeconds       int                `json:"timeout_seconds"`
		Retries              int                `json:"retries"`
		DNSServer            string             `json:"dns_server,omitempty"`
		DNSRecordType        string             `json:"dns_record_type,omitempty"`
		DNSExpected          string             `json:"dns_expected,omitempty"`
		HTTPAcceptedStatuses string             `json:"http_accepted_statuses,omitempty"`
		HTTPIgnoreTLS        bool               `json:"http_ignore_tls,omitempty"`
		HTTPMethod           string             `json:"http_method,omitempty"`
		HTTPKeyword          string             `json:"http_keyword,omitempty"`
		HTTPKeywordInvert    bool               `json:"http_keyword_invert,omitempty"`
		HTTPUsername         string             `json:"http_username,omitempty"`
		HTTPBearerToken      string             `json:"http_bearer_token,omitempty"`
		HTTPMaxRedirects     int                `json:"http_max_redirects,omitempty"`
		HTTPHeaderName       string             `json:"http_header_name,omitempty"`
		HTTPHeaderValue      string             `json:"http_header_value,omitempty"`
		HTTPBodyType         string             `json:"http_body_type,omitempty"`
		HTTPJsonPath         string             `json:"http_json_path,omitempty"`
		HTTPJsonExpected     string             `json:"http_json_expected,omitempty"`
		HTTPXPath            string             `json:"http_xpath,omitempty"`
		HTTPXPathExpected    string             `json:"http_xpath_expected,omitempty"`
		SMTPUseTLS           bool               `json:"smtp_use_tls,omitempty"`
		SMTPIgnoreTLS        bool               `json:"smtp_ignore_tls,omitempty"`
		SMTPUsername         string             `json:"smtp_username,omitempty"`
		// SMTPPassword and HTTPPassword intentionally excluded from exports.
		NotifyOnFailure bool `json:"notify_on_failure"`
		NotifyOnSuccess bool `json:"notify_on_success"`
		NotifyBodyChars int  `json:"notify_body_chars,omitempty"`
	}
	doc := exportDoc{
		Schema:               "service-monitor/monitor/v1",
		Name:                 m.Name,
		Type:                 m.Type,
		URL:                  m.URL,
		IntervalSeconds:      m.IntervalSeconds,
		TimeoutSeconds:       m.TimeoutSeconds,
		Retries:              m.Retries,
		DNSServer:            m.DNSServer,
		DNSRecordType:        m.DNSRecordType,
		DNSExpected:          m.DNSExpected,
		HTTPAcceptedStatuses: m.HTTPAcceptedStatuses,
		HTTPIgnoreTLS:        m.HTTPIgnoreTLS,
		HTTPMethod:           m.HTTPMethod,
		HTTPKeyword:          m.HTTPKeyword,
		HTTPKeywordInvert:    m.HTTPKeywordInvert,
		HTTPUsername:         m.HTTPUsername,
		HTTPBearerToken:      m.HTTPBearerToken,
		HTTPMaxRedirects:     m.HTTPMaxRedirects,
		HTTPHeaderName:       m.HTTPHeaderName,
		HTTPHeaderValue:      m.HTTPHeaderValue,
		HTTPBodyType:         m.HTTPBodyType,
		HTTPJsonPath:         m.HTTPJsonPath,
		HTTPJsonExpected:     m.HTTPJsonExpected,
		HTTPXPath:            m.HTTPXPath,
		HTTPXPathExpected:    m.HTTPXPathExpected,
		SMTPUseTLS:           m.SMTPUseTLS,
		SMTPIgnoreTLS:        m.SMTPIgnoreTLS,
		SMTPUsername:         m.SMTPUsername,
		NotifyOnFailure:      m.NotifyOnFailure,
		NotifyOnSuccess:      m.NotifyOnSuccess,
		NotifyBodyChars:      m.NotifyBodyChars,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": "Failed to encode monitor"})
		return
	}

	filename := fmt.Sprintf("monitor-%s.json", m.Name)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

// MonitorImport handles a JSON file upload, parses it, creates the monitor,
// and redirects to the edit page so the user can review before first run.
func (h *Handler) MonitorImport(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "No file uploaded"})
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, 1<<20)) // 1 MB max
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "Failed to read file"})
		return
	}

	type importDoc struct {
		Schema               string             `json:"schema"`
		Name                 string             `json:"name"`
		Type                 models.MonitorType `json:"type"`
		URL                  string             `json:"url"`
		IntervalSeconds      int                `json:"interval_seconds"`
		TimeoutSeconds       int                `json:"timeout_seconds"`
		Retries              int                `json:"retries"`
		DNSServer            string             `json:"dns_server"`
		DNSRecordType        string             `json:"dns_record_type"`
		DNSExpected          string             `json:"dns_expected"`
		HTTPAcceptedStatuses string             `json:"http_accepted_statuses"`
		HTTPIgnoreTLS        bool               `json:"http_ignore_tls"`
		HTTPMethod           string             `json:"http_method"`
		HTTPKeyword          string             `json:"http_keyword"`
		HTTPKeywordInvert    bool               `json:"http_keyword_invert"`
		HTTPUsername         string             `json:"http_username"`
		HTTPBearerToken      string             `json:"http_bearer_token"`
		HTTPMaxRedirects     int                `json:"http_max_redirects"`
		HTTPHeaderName       string             `json:"http_header_name"`
		HTTPHeaderValue      string             `json:"http_header_value"`
		HTTPBodyType         string             `json:"http_body_type"`
		HTTPJsonPath         string             `json:"http_json_path"`
		HTTPJsonExpected     string             `json:"http_json_expected"`
		HTTPXPath            string             `json:"http_xpath"`
		HTTPXPathExpected    string             `json:"http_xpath_expected"`
		SMTPUseTLS           bool               `json:"smtp_use_tls"`
		SMTPIgnoreTLS        bool               `json:"smtp_ignore_tls"`
		SMTPUsername         string             `json:"smtp_username"`
		NotifyOnFailure      bool               `json:"notify_on_failure"`
		NotifyOnSuccess      bool               `json:"notify_on_success"`
		NotifyBodyChars      int                `json:"notify_body_chars"`
	}

	var doc importDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "Invalid JSON: " + err.Error()})
		return
	}
	if doc.Name == "" || doc.URL == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "Imported file is missing required fields: name, url"})
		return
	}
	if doc.IntervalSeconds < 20 {
		doc.IntervalSeconds = 60
	}
	if doc.TimeoutSeconds < 1 {
		doc.TimeoutSeconds = 30
	}
	if doc.DNSRecordType == "" {
		doc.DNSRecordType = "A"
	}

	m := &models.Monitor{
		Name:                 doc.Name + " (imported)",
		Type:                 doc.Type,
		URL:                  doc.URL,
		IntervalSeconds:      doc.IntervalSeconds,
		TimeoutSeconds:       doc.TimeoutSeconds,
		Retries:              doc.Retries,
		Active:               false, // start paused so the user can review first
		DNSServer:            doc.DNSServer,
		DNSRecordType:        doc.DNSRecordType,
		DNSExpected:          doc.DNSExpected,
		HTTPAcceptedStatuses: doc.HTTPAcceptedStatuses,
		HTTPIgnoreTLS:        doc.HTTPIgnoreTLS,
		HTTPMethod:           doc.HTTPMethod,
		HTTPKeyword:          doc.HTTPKeyword,
		HTTPKeywordInvert:    doc.HTTPKeywordInvert,
		HTTPUsername:         doc.HTTPUsername,
		HTTPBearerToken:      doc.HTTPBearerToken,
		HTTPMaxRedirects:     doc.HTTPMaxRedirects,
		HTTPHeaderName:       doc.HTTPHeaderName,
		HTTPHeaderValue:      doc.HTTPHeaderValue,
		HTTPBodyType:         doc.HTTPBodyType,
		HTTPJsonPath:         doc.HTTPJsonPath,
		HTTPJsonExpected:     doc.HTTPJsonExpected,
		HTTPXPath:            doc.HTTPXPath,
		HTTPXPathExpected:    doc.HTTPXPathExpected,
		SMTPUseTLS:           doc.SMTPUseTLS,
		SMTPIgnoreTLS:        doc.SMTPIgnoreTLS,
		SMTPUsername:         doc.SMTPUsername,
		NotifyOnFailure:      doc.NotifyOnFailure,
		NotifyOnSuccess:      doc.NotifyOnSuccess,
		NotifyBodyChars:      doc.NotifyBodyChars,
		// SMTPPassword is not exported and must be re-entered after import.
	}

	id, err := h.monitors.Create(m)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": "Failed to save monitor: " + err.Error()})
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/monitors/%d/edit", id))
}

// MonitorPush handles an incoming heartbeat ping for a push-type monitor.
// This endpoint is intentionally unauthenticated — the random push token acts as the credential.
func (h *Handler) MonitorPush(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "token required"})
		return
	}
	m, err := h.monitors.GetByPushToken(token)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "monitor not found"})
		return
	}
	if !m.Active {
		c.JSON(http.StatusOK, gin.H{"ok": false, "msg": "monitor is paused"})
		return
	}
	h.sched.RecordHeartbeat(m, 1, 0, "push received")
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
	dnsRecordType := c.DefaultPostForm("dns_record_type", "A")
	dnsExpected := c.PostForm("dns_expected")

	// HTTP extended fields
	httpAcceptedStatuses := c.PostForm("http_accepted_statuses")
	httpIgnoreTLS := c.PostForm("http_ignore_tls") == "on"
	httpMethod := c.DefaultPostForm("http_method", "GET")
	httpKeyword := c.PostForm("http_keyword")
	httpKeywordInvert := c.PostForm("http_keyword_invert") == "on"
	httpUsername := c.PostForm("http_username")
	httpPassword := c.PostForm("http_password")
	httpBearerToken := c.PostForm("http_bearer_token")
	httpMaxRedirects, err3 := strconv.Atoi(c.DefaultPostForm("http_max_redirects", "10"))
	if err3 != nil || httpMaxRedirects < 0 {
		httpMaxRedirects = 10
	}

	// Push token is carried via a hidden form input so edits don't regenerate it.
	pushToken := c.PostForm("push_token")

	// Custom request fields
	httpRequestHeaders := c.PostForm("http_request_headers")
	httpRequestBody := c.PostForm("http_request_body")

	// Response assertion fields
	httpHeaderName := c.PostForm("http_header_name")
	httpHeaderValue := c.PostForm("http_header_value")
	httpBodyType := c.PostForm("http_body_type")
	httpJsonPath := c.PostForm("http_json_path")
	httpJsonExpected := c.PostForm("http_json_expected")
	httpXPath := c.PostForm("http_xpath")
	httpXPathExpected := c.PostForm("http_xpath_expected")

	// SMTP fields
	smtpUseTLS := c.PostForm("smtp_use_tls") == "on"
	smtpIgnoreTLS := c.PostForm("smtp_ignore_tls") == "on"
	smtpUsername := c.PostForm("smtp_username")
	smtpPassword := c.PostForm("smtp_password")

	// Notification trigger settings
	notifyOnFailure := c.PostForm("notify_on_failure") == "on"
	notifyOnSuccess := c.PostForm("notify_on_success") == "on"
	notifyBodyChars, err4 := strconv.Atoi(c.DefaultPostForm("notify_body_chars", "0"))
	if err4 != nil || notifyBodyChars < 0 {
		notifyBodyChars = 0
	}
	if notifyBodyChars > 4096 {
		notifyBodyChars = 4096
	}

	// Always build a partial monitor so error paths never get nil.
	m := &models.Monitor{
		Name:                 name,
		Type:                 monType,
		URL:                  monURL,
		IntervalSeconds:      intervalSec,
		TimeoutSeconds:       timeoutSec,
		Active:               true,
		Retries:              retries,
		DNSServer:            dnsServer,
		DNSRecordType:        dnsRecordType,
		DNSExpected:          dnsExpected,
		HTTPAcceptedStatuses: httpAcceptedStatuses,
		HTTPIgnoreTLS:        httpIgnoreTLS,
		HTTPMethod:           httpMethod,
		HTTPKeyword:          httpKeyword,
		HTTPKeywordInvert:    httpKeywordInvert,
		HTTPUsername:         httpUsername,
		HTTPPassword:         httpPassword,
		HTTPBearerToken:      httpBearerToken,
		HTTPMaxRedirects:     httpMaxRedirects,
		HTTPRequestHeaders:   httpRequestHeaders,
		HTTPRequestBody:      httpRequestBody,
		PushToken:            pushToken,
		HTTPHeaderName:       httpHeaderName,
		HTTPHeaderValue:      httpHeaderValue,
		HTTPBodyType:         httpBodyType,
		HTTPJsonPath:         httpJsonPath,
		HTTPJsonExpected:     httpJsonExpected,
		HTTPXPath:            httpXPath,
		HTTPXPathExpected:    httpXPathExpected,
		SMTPUseTLS:           smtpUseTLS,
		SMTPIgnoreTLS:        smtpIgnoreTLS,
		SMTPUsername:         smtpUsername,
		SMTPPassword:         smtpPassword,
		NotifyOnFailure:      notifyOnFailure,
		NotifyOnSuccess:      notifyOnSuccess,
		NotifyBodyChars:      notifyBodyChars,
	}
	if name == "" {
		return m, &formError{"name is required"}
	}
	if monURL == "" && monType != models.MonitorTypePush {
		return m, &formError{"url is required"}
	}
	return m, nil
}

type formError struct{ msg string }

func (e *formError) Error() string { return e.msg }

// generatePushToken returns a random 16-byte hex string for push monitor authentication.
func generatePushToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail on a healthy OS; fall back to timestamp.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

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
		case "slack":
			if u := cfg["url"]; u != "" {
				if parsed, err := neturl.Parse(u); err == nil && parsed.Host != "" {
					summaries[n.ID] = parsed.Host
				} else {
					summaries[n.ID] = u
				}
			}
		case "discord":
			summaries[n.ID] = "Discord Webhook"
		case "ntfy":
			if topic := cfg["topic"]; topic != "" {
				summaries[n.ID] = "topic: " + topic
			}
		}
	}
	return summaries
}
