package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/models"
)

// generateUUID returns a random RFC 4122 version-4 UUID.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func (h *Handler) statusPageStore(c *gin.Context) *models.StatusPageStore {
	return models.NewStatusPageStore(h.userDB(c))
}

// StatusPageList renders the status pages management list.
func (h *Handler) StatusPageList(c *gin.Context) {
	pages, _ := h.statusPageStore(c).List()
	flash, _ := c.Cookie("sm_flash")
	if flash != "" {
		c.SetCookie("sm_flash", "", -1, "/", "", false, true)
	}
	c.HTML(http.StatusOK, "status_page_list.gohtml", h.pageData(c, gin.H{
		"Pages": pages,
		"Flash": flash,
	}))
}

// StatusPageNew renders the create form.
func (h *Handler) StatusPageNew(c *gin.Context) {
	monitors, _ := h.monitorStore(c).List()
	c.HTML(http.StatusOK, "status_page_form.gohtml", h.pageData(c, gin.H{
		"Page":             &models.StatusPage{},
		"IsNew":            true,
		"AllMonitors":      monitors,
		"LinkedMonitorIDs": map[int64]bool{},
		"Error":            "",
	}))
}

// StatusPageCreate handles the create form submission.
func (h *Handler) StatusPageCreate(c *gin.Context) {
	page, monitorIDs, err := statusPageFromForm(c)
	if err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusBadRequest, "status_page_form.gohtml", gin.H{
			"Page": page, "IsNew": true, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}

	spStore := h.statusPageStore(c)
	id, err := spStore.Create(page)
	if err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusInternalServerError, "status_page_form.gohtml", gin.H{
			"Page": page, "IsNew": true, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}

	// best-effort: register slug and UUID in shared DB for public endpoint lookups
	_ = h.users.RegisterStatusPageSlug(page.Slug, h.username(c), page.Name)
	_ = h.users.RegisterSummaryToken(page.SummaryUUID, h.username(c))
	_ = spStore.SetMonitors(id, monitorIDs)
	c.Redirect(http.StatusFound, "/status-pages")
}

// StatusPageEdit renders the edit form.
func (h *Handler) StatusPageEdit(c *gin.Context) {
	page, ok := h.getStatusPage(c)
	if !ok {
		return
	}
	spStore := h.statusPageStore(c)
	monitors, _ := h.monitorStore(c).List()
	linkedIDs, _ := spStore.ListMonitorIDs(page.ID)
	linked := make(map[int64]bool, len(linkedIDs))
	for _, id := range linkedIDs {
		linked[id] = true
	}
	c.HTML(http.StatusOK, "status_page_form.gohtml", h.pageData(c, gin.H{
		"Page":             page,
		"IsNew":            false,
		"AllMonitors":      monitors,
		"LinkedMonitorIDs": linked,
		"Error":            "",
	}))
}

// StatusPageUpdate handles the edit form submission.
func (h *Handler) StatusPageUpdate(c *gin.Context) {
	existing, ok := h.getStatusPage(c)
	if !ok {
		return
	}
	updated, monitorIDs, err := statusPageFromForm(c)
	if err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusBadRequest, "status_page_form.gohtml", gin.H{
			"Page": existing, "IsNew": false, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}
	updated.ID = existing.ID

	spStore := h.statusPageStore(c)
	if err := spStore.Update(updated); err != nil {
		monitors, _ := h.monitorStore(c).List()
		c.HTML(http.StatusInternalServerError, "status_page_form.gohtml", gin.H{
			"Page": updated, "IsNew": false, "AllMonitors": monitors,
			"LinkedMonitorIDs": map[int64]bool{}, "Error": err.Error(),
		})
		return
	}
	// Synchronise the shared slug + UUID indexes. best-effort — per-user DB is the source of truth.
	if existing.Slug != updated.Slug {
		if existing.Slug != "" {
			_ = h.users.UnregisterStatusPageSlug(existing.Slug)
		}
		if updated.Slug != "" {
			_ = h.users.RegisterStatusPageSlug(updated.Slug, h.username(c), updated.Name)
		}
	} else if updated.Slug != "" && existing.Name != updated.Name {
		// Slug unchanged but name changed — keep the index entry current.
		_ = h.users.RegisterStatusPageSlug(updated.Slug, h.username(c), updated.Name)
	}
	if existing.SummaryUUID != updated.SummaryUUID {
		if existing.SummaryUUID != "" {
			_ = h.users.UnregisterSummaryToken(existing.SummaryUUID)
		}
		if updated.SummaryUUID != "" {
			_ = h.users.RegisterSummaryToken(updated.SummaryUUID, h.username(c))
		}
	}
	_ = spStore.SetMonitors(updated.ID, monitorIDs)
	c.Redirect(http.StatusFound, "/status-pages")
}

// StatusPageDelete removes a status page.
func (h *Handler) StatusPageDelete(c *gin.Context) {
	page, ok := h.getStatusPage(c)
	if !ok {
		return
	}
	if page.Slug != "" {
		_ = h.users.UnregisterStatusPageSlug(page.Slug)
	}
	if page.SummaryUUID != "" {
		// best-effort: clean up shared index
		_ = h.users.UnregisterSummaryToken(page.SummaryUUID)
	}
	if err := h.statusPageStore(c).Delete(page.ID); err != nil {
		c.HTML(http.StatusInternalServerError, "error.gohtml", gin.H{"Error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, "/status-pages")
}

// statusPageCacheTTL controls how long the rendered public status page HTML is cached.
const statusPageCacheTTL = 60 * time.Second

// StatusPagePublic renders the unauthenticated public status page.
// The rendered HTML is cached for statusPageCacheTTL to protect the DB from
// repeated full-page loads (each page hit runs N×heartbeat queries).
// Route: GET /status/:slug
func (h *Handler) StatusPagePublic(c *gin.Context) {
	slug := c.Param("slug")

	cacheKey := "page\x00" + slug
	if cached, hit := h.pageCache.get(cacheKey); hit {
		c.Data(http.StatusOK, "text/html; charset=utf-8", cached)
		return
	}

	// Resolve slug → username via the shared users DB.
	username, err := h.users.LookupStatusPageSlug(slug)
	if err != nil || username == "" {
		c.HTML(http.StatusNotFound, "error.gohtml", gin.H{"Error": "Status page not found"})
		return
	}

	db, err := h.registry.Get(username)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.gohtml", gin.H{"Error": "Status page not found"})
		return
	}

	spStore := models.NewStatusPageStore(db)
	page, err := spStore.GetBySlug(slug)
	if err != nil || page == nil {
		c.HTML(http.StatusNotFound, "error.gohtml", gin.H{"Error": "Status page not found"})
		return
	}

	monitorIDs, _ := spStore.ListMonitorIDs(page.ID)
	mStore := models.NewMonitorStore(db)
	bStore := models.NewHeartbeatStore(db)

	type entry struct {
		Monitor      *models.Monitor
		Uptime24h    float64
		LatestStatus int
		Sparkline    template.HTML
	}
	now := time.Now().UTC()
	var monitors []entry
	allOperational := true
	for _, mid := range monitorIDs {
		m, err := mStore.Get(mid)
		if err != nil || m == nil {
			continue
		}
		latestStatus := -1 // -1 = pending/unknown
		beats, _ := bStore.Latest(m.ID, 50)
		if len(beats) > 0 {
			m.LastStatus = &beats[0].Status
			m.LastLatency = &beats[0].LatencyMs
			latestStatus = beats[0].Status
		}
		if latestStatus != 1 {
			allOperational = false
		}
		uptime24h, _ := bStore.UptimePercent(m.ID, now.Add(-24*time.Hour))
		monitors = append(monitors, entry{
			Monitor:      m,
			Uptime24h:    uptime24h,
			LatestStatus: latestStatus,
			Sparkline:    computeSparklineSVG(beats),
		})
	}

	templateData := gin.H{
		"Page":           page,
		"Monitors":       monitors,
		"AllOperational": allOperational && len(monitors) > 0,
		"Now":            now.Format("2006-01-02 15:04:05 UTC"),
		"UUID":           page.SummaryUUID,
	}

	// Render into a buffer so we can cache the result.
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, "status_page_public.gohtml", templateData); err != nil {
		// Template error — fall back to direct render (won't cache).
		c.HTML(http.StatusInternalServerError, "error.gohtml", gin.H{"Error": err.Error()})
		return
	}
	rendered := buf.Bytes()
	h.pageCache.set(cacheKey, rendered, statusPageCacheTTL)
	c.Data(http.StatusOK, "text/html; charset=utf-8", rendered)
}

// StatusPagePublicChartData is a public JSON endpoint that returns heartbeat
// history for a monitor, but only if it belongs to the given status page.
// Route: GET /status/:slug/chart-data/:id
func (h *Handler) StatusPagePublicChartData(c *gin.Context) {
	slug := c.Param("slug")
	idStr := c.Param("id")
	monitorID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monitor id"})
		return
	}

	// Resolve slug → username via the shared users DB.
	username, err := h.users.LookupStatusPageSlug(slug)
	if err != nil || username == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	db, err := h.registry.Get(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// Verify the monitor is actually linked to this status page.
	spStore := models.NewStatusPageStore(db)
	page, err := spStore.GetBySlug(slug)
	if err != nil || page == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	linkedIDs, _ := spStore.ListMonitorIDs(page.ID)
	found := false
	for _, id := range linkedIDs {
		if id == monitorID {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	span := c.DefaultQuery("since", "24h")
	dur, ok := allowedChartSpans[span]
	if !ok {
		dur = 24 * time.Hour
		span = "24h"
	}

	// Public callers get a cached response — protects the DB from flooding.
	// Authenticated owners (MonitorChartData) bypass this endpoint entirely.
	cacheKey := chartCacheKey(slug, idStr, span)
	if cached, hit := h.chartCache.get(cacheKey); hit {
		c.Data(http.StatusOK, "application/json; charset=utf-8", cached)
		return
	}

	beats, err := models.NewHeartbeatStore(db).LatestSince(monitorID, time.Now().Add(-dur), 500)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type point struct {
		TS      string `json:"ts"`
		Latency int    `json:"latency"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	// beats is newest-first; reverse to oldest-first for chart rendering
	pts := make([]point, len(beats))
	for i, b := range beats {
		pts[len(beats)-1-i] = point{
			TS:      b.CreatedAt.UTC().Format(time.RFC3339),
			Latency: b.LatencyMs,
			Status:  b.Status,
			Message: b.Message,
		}
	}

	type downtimeBand struct {
		Start string  `json:"start"`
		End   *string `json:"end"`
	}
	dtEvents, _ := models.NewDowntimeEventStore(db).ListSince(monitorID, time.Now().Add(-dur))
	bands := make([]downtimeBand, len(dtEvents))
	for i, e := range dtEvents {
		var end *string
		if e.EndedAt != nil {
			s := e.EndedAt.UTC().Format(time.RFC3339)
			end = &s
		}
		bands[i] = downtimeBand{Start: e.StartedAt.UTC().Format(time.RFC3339), End: end}
	}

	payload, err := json.Marshal(gin.H{"points": pts, "downtime": bands})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.chartCache.set(cacheKey, payload, chartCacheTTL)
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

func (h *Handler) getStatusPage(c *gin.Context) (*models.StatusPage, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.gohtml", gin.H{"Error": "invalid status page id"})
		return nil, false
	}
	page, err := h.statusPageStore(c).Get(id)
	if err != nil || page == nil {
		c.HTML(http.StatusNotFound, "error.gohtml", gin.H{"Error": "status page not found"})
		return nil, false
	}
	return page, true
}

// statusPageFromForm parses a status page and the linked monitor IDs from a POST form.
func statusPageFromForm(c *gin.Context) (*models.StatusPage, []int64, error) {
	name := c.PostForm("name")
	slug := c.PostForm("slug")
	desc := c.PostForm("description")

	// summary_uuid hidden field carries the existing UUID when editing.
	// Auto-generate a UUID on first save so every page has a stable public URL.
	summaryUUID := c.PostForm("summary_uuid")
	if summaryUUID == "" {
		summaryUUID = generateUUID()
	}

	page := &models.StatusPage{Name: name, Slug: slug, Description: desc, SummaryUUID: summaryUUID}
	if name == "" {
		return page, nil, &formError{"name is required"}
	}
	if slug == "" {
		return page, nil, &formError{"slug is required"}
	}

	var monitorIDs []int64
	for _, v := range c.PostFormArray("monitors") {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			monitorIDs = append(monitorIDs, id)
		}
	}
	return page, monitorIDs, nil
}

// summaryCacheTTL controls how long the public JSON summary response is cached.
const summaryCacheTTL = 60 * time.Second

// StatusPagePublicSummary serves a machine-readable JSON summary for a status page.
// The page must have the JSON API feature enabled (summary_uuid != "").
// Route: GET /summary/:uuid
func (h *Handler) StatusPagePublicSummary(c *gin.Context) {
	uuid := c.Param("uuid")

	cacheKey := "summary\x00" + uuid
	if cached, hit := h.pageCache.get(cacheKey); hit {
		c.Data(http.StatusOK, "application/json; charset=utf-8", cached)
		return
	}

	// Resolve UUID → username via the shared users DB.
	username, err := h.users.LookupSummaryToken(uuid)
	if err != nil || username == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	db, err := h.registry.Get(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	spStore := models.NewStatusPageStore(db)
	page, err := spStore.GetBySummaryUUID(uuid)
	if err != nil || page == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	monitorIDs, _ := spStore.ListMonitorIDs(page.ID)
	mStore := models.NewMonitorStore(db)
	bStore := models.NewHeartbeatStore(db)

	type monitorEntry struct {
		ID        int64   `json:"id"`
		Name      string  `json:"name"`
		Status    int     `json:"status"`
		Uptime24h float64 `json:"uptime_24h"`
	}

	now := time.Now().UTC()
	allOperational := true
	monitors := make([]monitorEntry, 0, len(monitorIDs))
	for _, mid := range monitorIDs {
		m, err := mStore.Get(mid)
		if err != nil || m == nil {
			continue
		}
		latestStatus := -1
		beats, _ := bStore.Latest(m.ID, 1)
		if len(beats) > 0 {
			latestStatus = beats[0].Status
		}
		if latestStatus != 1 {
			allOperational = false
		}
		uptime24h, _ := bStore.UptimePercent(m.ID, now.Add(-24*time.Hour))
		monitors = append(monitors, monitorEntry{
			ID:        m.ID,
			Name:      m.Name,
			Status:    latestStatus,
			Uptime24h: uptime24h,
		})
	}

	type summaryResponse struct {
		Name           string         `json:"name"`
		Slug           string         `json:"slug"`
		Description    string         `json:"description"`
		AllOperational bool           `json:"all_operational"`
		Monitors       []monitorEntry `json:"monitors"`
		GeneratedAt    string         `json:"generated_at"`
	}

	payload, err := json.Marshal(summaryResponse{
		Name:           page.Name,
		Slug:           page.Slug,
		Description:    page.Description,
		AllOperational: allOperational && len(monitors) > 0,
		Monitors:       monitors,
		GeneratedAt:    now.Format(time.RFC3339),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.pageCache.set(cacheKey, payload, summaryCacheTTL)
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}
