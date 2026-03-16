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
	allNotifs, _ := h.notifStore(c).List()
	allTags, _ := h.tagStore(c).List()
	c.HTML(http.StatusOK, "monitor_form.html", h.pageData(c, gin.H{
		"Monitor":        &models.Monitor{IntervalSeconds: 60, TimeoutSeconds: 30, Retries: 1, NotifyOnFailure: true, NotifyOnSuccess: true},
		"IsNew":          true,
		"Error":          "",
		"AllNotifs":      allNotifs,
		"LinkedNotifIDs": map[int64]bool{},
		"NotifSummaries": notifSummaryMap(allNotifs),
		"AllTags":        allTags,
		"LinkedTagIDs":   map[int64]bool{},
	}))
}

// MonitorCreate handles new monitor form submission.
func (h *Handler) MonitorCreate(c *gin.Context) {
	m, err := monitorFromForm(c)
	if err != nil {
		allNotifs, _ := h.notifStore(c).List()
		allTags, _ := h.tagStore(c).List()
		c.HTML(http.StatusBadRequest, "monitor_form.html", gin.H{
			"Monitor": m, "IsNew": true, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": map[int64]bool{},
			"NotifSummaries": notifSummaryMap(allNotifs),
			"AllTags":        allTags, "LinkedTagIDs": map[int64]bool{},
		})
		return
	}

	// Ensure push monitors always have a token.
	if m.Type == models.MonitorTypePush && m.PushToken == "" {
		m.PushToken = generatePushToken()
	}

	id, err := h.monitorStore(c).Create(m)
	if err != nil {
		allNotifs, _ := h.notifStore(c).List()
		allTags, _ := h.tagStore(c).List()
		c.HTML(http.StatusInternalServerError, "monitor_form.html", gin.H{
			"Monitor": m, "IsNew": true, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": map[int64]bool{},
			"NotifSummaries": notifSummaryMap(allNotifs),
			"AllTags":        allTags, "LinkedTagIDs": map[int64]bool{},
		})
		return
	}

	m.ID = id
	_ = h.notifStore(c).ReplaceMonitorLinks(m.ID, notifIDsFromForm(c))
	_ = h.tagStore(c).SetMonitorTags(m.ID, tagIDsFromForm(c))

	// Register the push token in the shared users DB so the unauthenticated
	// /push/:token endpoint can resolve which user's DB to look in.
	if m.Type == models.MonitorTypePush && m.PushToken != "" {
		_ = h.users.RegisterPushToken(m.PushToken, h.username(c))
	}

	h.schedFor(c).Schedule(m)
	c.Redirect(http.StatusFound, "/")
}

// MonitorDetail renders a monitor's heartbeat history.
func (h *Handler) MonitorDetail(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}

	bstore := h.heartbeatStore(c)
	beats, _ := bstore.Latest(m.ID, 100)
	uptime24h, _ := bstore.UptimePercent(m.ID, time.Now().Add(-24*time.Hour))
	uptime30d, _ := bstore.UptimePercent(m.ID, time.Now().Add(-30*24*time.Hour))

	c.HTML(http.StatusOK, "monitor_detail.html", h.pageData(c, gin.H{
		"Monitor":   m,
		"Beats":     beats,
		"Uptime24h": uptime24h,
		"Uptime30d": uptime30d,
	}))
}

// MonitorEdit renders the edit form for an existing monitor.
func (h *Handler) MonitorEdit(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	nstore := h.notifStore(c)
	allNotifs, _ := nstore.List()
	linked, _ := nstore.ListForMonitor(m.ID)
	linkedIDs := make(map[int64]bool, len(linked))
	for _, n := range linked {
		linkedIDs[n.ID] = true
	}
	tstore := h.tagStore(c)
	allTags, _ := tstore.List()
	linkedTags, _ := tstore.ListForMonitor(m.ID)
	linkedTagIDs := make(map[int64]bool, len(linkedTags))
	for _, t := range linkedTags {
		linkedTagIDs[t.ID] = true
	}
	c.HTML(http.StatusOK, "monitor_form.html", h.pageData(c, gin.H{
		"Monitor":        m,
		"IsNew":          false,
		"Error":          "",
		"AllNotifs":      allNotifs,
		"LinkedNotifIDs": linkedIDs,
		"NotifSummaries": notifSummaryMap(allNotifs),
		"AllTags":        allTags,
		"LinkedTagIDs":   linkedTagIDs,
	}))
}

// MonitorUpdate handles the edit form submission.
func (h *Handler) MonitorUpdate(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}

	nstore := h.notifStore(c)
	updated, err := monitorFromForm(c)
	if err != nil {
		allNotifs, _ := nstore.List()
		linked, _ := nstore.ListForMonitor(m.ID)
		linkedIDs := make(map[int64]bool, len(linked))
		for _, n := range linked {
			linkedIDs[n.ID] = true
		}
		allTags, _ := h.tagStore(c).List()
		linkedTags, _ := h.tagStore(c).ListForMonitor(m.ID)
		linkedTagIDs := make(map[int64]bool, len(linkedTags))
		for _, t := range linkedTags {
			linkedTagIDs[t.ID] = true
		}
		c.HTML(http.StatusBadRequest, "monitor_form.html", gin.H{
			"Monitor": m, "IsNew": false, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": linkedIDs,
			"NotifSummaries": notifSummaryMap(allNotifs),
			"AllTags":        allTags, "LinkedTagIDs": linkedTagIDs,
		})
		return
	}
	updated.ID = m.ID

	// Manage push token registration in the shared users DB.
	if m.Type == models.MonitorTypePush && m.PushToken != "" {
		// Unregister old token if type changes away from push or token changes.
		if updated.Type != models.MonitorTypePush || updated.PushToken != m.PushToken {
			_ = h.users.UnregisterPushToken(m.PushToken)
		}
	}
	if updated.Type == models.MonitorTypePush {
		if updated.PushToken == "" {
			updated.PushToken = generatePushToken()
		}
		_ = h.users.RegisterPushToken(updated.PushToken, h.username(c))
	}

	if err := h.monitorStore(c).Update(updated); err != nil {
		allNotifs, _ := nstore.List()
		allTags, _ := h.tagStore(c).List()
		c.HTML(http.StatusInternalServerError, "monitor_form.html", gin.H{
			"Monitor": updated, "IsNew": false, "Error": err.Error(),
			"AllNotifs": allNotifs, "LinkedNotifIDs": map[int64]bool{},
			"NotifSummaries": notifSummaryMap(allNotifs),
			"AllTags":        allTags, "LinkedTagIDs": map[int64]bool{},
		})
		return
	}

	_ = nstore.ReplaceMonitorLinks(updated.ID, notifIDsFromForm(c))
	_ = h.tagStore(c).SetMonitorTags(updated.ID, tagIDsFromForm(c))
	h.schedFor(c).Schedule(updated)
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
		DBQuery         string `json:"db_query,omitempty"`
		NotifyOnFailure bool   `json:"notify_on_failure"`
		NotifyOnSuccess bool   `json:"notify_on_success"`
		NotifyBodyChars int    `json:"notify_body_chars,omitempty"`
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
		DBQuery:              m.DBQuery,
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
		DBQuery              string             `json:"db_query"`
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
		DBQuery:              doc.DBQuery,
		NotifyOnFailure:      doc.NotifyOnFailure,
		NotifyOnSuccess:      doc.NotifyOnSuccess,
		NotifyBodyChars:      doc.NotifyBodyChars,
		// SMTPPassword is not exported and must be re-entered after import.
	}

	id, err := h.monitorStore(c).Create(m)
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

	// Resolve the owning user from the shared push_tokens table.
	username, err := h.users.LookupPushToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "monitor not found"})
		return
	}

	db, err := h.registry.Get(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "internal error"})
		return
	}

	mstore := models.NewMonitorStore(db)
	m, err := mstore.GetByPushToken(token)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "monitor not found"})
		return
	}
	if !m.Active {
		c.JSON(http.StatusOK, gin.H{"ok": false, "msg": "monitor is paused"})
		return
	}

	sched := h.msched.ForUser(username)
	if sched == nil {
		// Scheduler not yet running — start it lazily.
		h.msched.StartForUser(username, db)
		sched = h.msched.ForUser(username)
	}
	sched.RecordHeartbeat(m, 1, 0, "push received")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MonitorDelete removes a monitor.
func (h *Handler) MonitorDelete(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	// Unregister push token from the shared users DB.
	if m.Type == models.MonitorTypePush && m.PushToken != "" {
		_ = h.users.UnregisterPushToken(m.PushToken)
	}
	h.schedFor(c).Unschedule(m.ID)
	if err := h.monitorStore(c).Delete(m.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, "/")
}

// MonitorPause pauses a monitor.
func (h *Handler) MonitorPause(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	if err := h.monitorStore(c).SetActive(m.ID, false); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	h.schedFor(c).Unschedule(m.ID)
	c.Redirect(http.StatusFound, "/")
}

// MonitorResume resumes a paused monitor.
func (h *Handler) MonitorResume(c *gin.Context) {
	m, ok := h.getMonitor(c)
	if !ok {
		return
	}
	if err := h.monitorStore(c).SetActive(m.ID, true); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}
	m.Active = true
	h.schedFor(c).Schedule(m)
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

	m, err := h.monitorStore(c).Get(id)
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

	// Database monitor fields
	dbQuery := c.PostForm("db_query")

	// Notification trigger settings
	notifyOnFailure := c.PostForm("notify_on_failure") == "on"
	notifyOnSuccess := c.PostForm("notify_on_success") == "on"
	notifyBodyChars, err4 := strconv.Atoi(c.DefaultPostForm("notify_body_chars", "0"))
	if err4 != nil || notifyBodyChars < 0 {
		notifyBodyChars = 0
	}
	certExpiryAlertDays, _ := strconv.Atoi(c.DefaultPostForm("cert_expiry_alert_days", "0"))
	if certExpiryAlertDays < 0 {
		certExpiryAlertDays = 0
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
		DBQuery:              dbQuery,
		NotifyOnFailure:      notifyOnFailure,
		NotifyOnSuccess:      notifyOnSuccess,
		NotifyBodyChars:      notifyBodyChars,
		CertExpiryAlertDays:  certExpiryAlertDays,
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

// tagIDsFromForm parses the repeated "tag_ids" form values into a slice of int64 IDs.
func tagIDsFromForm(c *gin.Context) []int64 {
	vals := c.PostFormArray("tag_ids")
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
