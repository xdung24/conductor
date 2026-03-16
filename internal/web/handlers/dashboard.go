package handlers

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/xdung24/service-monitor/internal/config"
	"github.com/xdung24/service-monitor/internal/database"
	"github.com/xdung24/service-monitor/internal/models"
	"github.com/xdung24/service-monitor/internal/scheduler"
	"golang.org/x/crypto/bcrypt"
)

// sparklineTmpl is parsed once at package init; html/template auto-escapes all dynamic values.
var sparklineTmpl = template.Must(template.New("sparkline").Parse(
	`<svg width="{{.W}}" height="{{.H}}" viewBox="0 0 {{.W}} {{.H}}" xmlns="http://www.w3.org/2000/svg" style="display:block;">` +
		`<polyline points="{{.Pts}}" fill="none" stroke="#38bdf8" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/></svg>`,
))

const sessionCookieName = "sm_session"

// Context keys injected by AuthRequired middleware.
const (
	ctxKeyUserDB   = "sm_user_db"
	ctxKeyUsername = "sm_username"
	ctxKeyIsAdmin  = "sm_is_admin"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	usersDB  *sql.DB                   // shared users database (auth + push_tokens)
	registry *database.Registry        // per-user data DB registry
	msched   *scheduler.MultiScheduler // per-user schedulers
	cfg      *config.Config
	users    *models.UserStore // backed by usersDB
}

// New creates a Handler.
func New(usersDB *sql.DB, registry *database.Registry, msched *scheduler.MultiScheduler, cfg *config.Config) *Handler {
	return &Handler{
		usersDB:  usersDB,
		registry: registry,
		msched:   msched,
		cfg:      cfg,
		users:    models.NewUserStore(usersDB),
	}
}

// ---------------------------------------------------------------------------
// Per-request context helpers
// ---------------------------------------------------------------------------

func (h *Handler) userDB(c *gin.Context) *sql.DB {
	db, _ := c.Get(ctxKeyUserDB)
	return db.(*sql.DB)
}

func (h *Handler) username(c *gin.Context) string {
	return c.GetString(ctxKeyUsername)
}

func (h *Handler) monitorStore(c *gin.Context) *models.MonitorStore {
	return models.NewMonitorStore(h.userDB(c))
}

func (h *Handler) heartbeatStore(c *gin.Context) *models.HeartbeatStore {
	return models.NewHeartbeatStore(h.userDB(c))
}

func (h *Handler) notifStore(c *gin.Context) *models.NotificationStore {
	return models.NewNotificationStore(h.userDB(c))
}

func (h *Handler) notifLogStore(c *gin.Context) *models.NotificationLogStore {
	return models.NewNotificationLogStore(h.userDB(c))
}

func (h *Handler) schedFor(c *gin.Context) *scheduler.Scheduler {
	return h.msched.ForUser(h.username(c))
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// AuthRequired is a Gin middleware that ensures the caller is authenticated.
// It accepts both a session cookie (browser login) and an
// "Authorization: Bearer <api-key>" header (programmatic access).
func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// --- Bearer token (API key) ---
		if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			plainToken := strings.TrimPrefix(authHeader, "Bearer ")
			username, err := h.apiKeyStore().Verify(plainToken)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
				return
			}
			db, err := h.registry.Get(username)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			u, err := h.users.GetByUsername(username)
			if err != nil || u == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
				return
			}
			c.Set(ctxKeyUserDB, db)
			c.Set(ctxKeyUsername, username)
			c.Set(ctxKeyIsAdmin, u.IsAdmin)
			c.Next()
			return
		}

		// --- Session cookie ---
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || !h.validSession(cookie) {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		username, _ := verifyToken(cookie, h.cfg.SecretKey)
		db, err := h.registry.Get(username)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": "failed to open user database"})
			c.Abort()
			return
		}
		u, err := h.users.GetByUsername(username)
		if err != nil || u == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set(ctxKeyUserDB, db)
		c.Set(ctxKeyUsername, username)
		c.Set(ctxKeyIsAdmin, u.IsAdmin)
		c.Next()
	}
}

// AdminRequired is a Gin middleware that allows only admin users.
// Must be used after AuthRequired.
func (h *Handler) AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !c.GetBool(ctxKeyIsAdmin) {
			c.HTML(http.StatusForbidden, "error.html", gin.H{"Error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// isAdmin reports whether the current request is from an admin user.
func (h *Handler) isAdmin(c *gin.Context) bool {
	return c.GetBool(ctxKeyIsAdmin)
}

// pageData returns a gin.H with common authenticated-page fields (IsAdmin)
// merged with any page-specific fields supplied by the caller.
func (h *Handler) pageData(c *gin.Context, extra gin.H) gin.H {
	data := gin.H{
		"IsAdmin": h.isAdmin(c),
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

func (h *Handler) validSession(token string) bool {
	username, ok := verifyToken(token, h.cfg.SecretKey)
	if !ok {
		return false
	}
	u, err := h.users.GetByUsername(username)
	return err == nil && u != nil
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// LoginPage renders the login form.
// When no users exist yet, a notice is shown directing the operator to the
// console-printed setup URL.
func (h *Handler) LoginPage(c *gin.Context) {
	count, _ := h.users.Count()
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Error":     "",
		"SetupMode": count == 0,
	})
}

// LoginSubmit validates credentials and sets a session cookie.
// If the user has 2FA enabled, a short-lived pending cookie is issued instead
// and the browser is redirected to the TOTP verification step.
func (h *Handler) LoginSubmit(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	u, err := h.users.GetByUsername(username)
	if err != nil || u == nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid username or password"})
		return
	}

	// If 2FA is enabled, issue a short-lived pending token and redirect to the TOTP step.
	_, enabled, _ := h.users.GetTOTP(username)
	if enabled {
		pendingToken := signPendingToken(username, h.cfg.SecretKey)
		c.SetCookie("sm_pending", pendingToken, int(5*time.Minute/time.Second), "/", "", false, true)
		c.Redirect(http.StatusFound, "/login/2fa")
		return
	}

	token := signToken(username, h.cfg.SecretKey)
	c.SetCookie(sessionCookieName, token, int(24*time.Hour/time.Second), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

// TwoFALoginPage renders the TOTP verification step of the login flow.
func (h *Handler) TwoFALoginPage(c *gin.Context) {
	pending, err := c.Cookie("sm_pending")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	_, ok := verifyPendingToken(pending, h.cfg.SecretKey)
	if !ok {
		c.SetCookie("sm_pending", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	}
	c.HTML(http.StatusOK, "login_2fa.html", gin.H{"Error": ""})
}

// TwoFALoginSubmit validates the TOTP code and completes the login.
func (h *Handler) TwoFALoginSubmit(c *gin.Context) {
	pending, err := c.Cookie("sm_pending")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	username, ok := verifyPendingToken(pending, h.cfg.SecretKey)
	if !ok {
		c.SetCookie("sm_pending", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	}

	code := strings.TrimSpace(c.PostForm("code"))
	secret, enabled, totpErr := h.users.GetTOTP(username)
	if totpErr != nil || !enabled || secret == "" {
		c.SetCookie("sm_pending", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if !totp.Validate(code, secret) {
		c.HTML(http.StatusUnauthorized, "login_2fa.html", gin.H{"Error": "Invalid code. Please try again."})
		return
	}

	c.SetCookie("sm_pending", "", -1, "/", "", false, true)
	token := signToken(username, h.cfg.SecretKey)
	c.SetCookie(sessionCookieName, token, int(24*time.Hour/time.Second), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

// Logout clears the session cookie.
func (h *Handler) Logout(c *gin.Context) {
	c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

// Dashboard renders the main monitor list page.
func (h *Handler) Dashboard(c *gin.Context) {
	mstore := h.monitorStore(c)
	bstore := h.heartbeatStore(c)
	tstore := h.tagStore(c)

	monitors, err := mstore.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}

	now := time.Now()
	monitorIDs := make([]int64, 0, len(monitors))
	for _, m := range monitors {
		monitorIDs = append(monitorIDs, m.ID)
	}

	sparklines := make(map[int64]template.HTML, len(monitors))
	tagMap, _ := tstore.TagMapForMonitors(monitorIDs)

	for _, m := range monitors {
		beats, _ := bstore.Latest(m.ID, 50)
		if len(beats) > 0 {
			m.LastStatus = &beats[0].Status
			m.LastLatency = &beats[0].LatencyMs
			m.LastMessage = &beats[0].Message
		}
		m.Uptime24h, _ = bstore.UptimePercent(m.ID, now.Add(-24*time.Hour))
		m.Uptime30d, _ = bstore.UptimePercent(m.ID, now.Add(-30*24*time.Hour))
		sparklines[m.ID] = computeSparklineSVG(beats)
	}

	c.HTML(http.StatusOK, "dashboard.html", h.pageData(c, gin.H{
		"Monitors":   monitors,
		"Sparklines": sparklines,
		"Tags":       tagMap,
		"Username":   h.username(c),
	}))
}

// computeSparklineSVG generates an inline SVG polyline from heartbeat data.
// beats should be in newest-first order (as returned by bstore.Latest).
func computeSparklineSVG(beats []*models.Heartbeat) template.HTML {
	if len(beats) == 0 {
		return ""
	}

	// Reverse to oldest-first for left-to-right rendering.
	n := len(beats)
	points := make([]int, n)
	for i, b := range beats {
		points[n-1-i] = b.LatencyMs
	}

	// Find max latency for scaling (minimum 1 to avoid divide-by-zero).
	maxVal := 1
	for _, v := range points {
		if v > maxVal {
			maxVal = v
		}
	}

	const svgW, svgH = 80, 24
	var pts string
	for i, v := range points {
		x := float64(i) * float64(svgW) / float64(len(points)-1)
		y := float64(svgH) - (float64(v)/float64(maxVal))*float64(svgH-2) - 1
		if i == 0 {
			pts += fmt.Sprintf("%.1f,%.1f", x, y)
		} else {
			pts += fmt.Sprintf(" %.1f,%.1f", x, y)
		}
	}

	var buf bytes.Buffer
	if err := sparklineTmpl.Execute(&buf, struct {
		W, H int
		Pts  string
	}{svgW, svgH, pts}); err != nil {
		return ""
	}
	return template.HTML(buf.String()) //nolint:gosec // content produced by html/template; all dynamic values are numeric
}
