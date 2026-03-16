package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/config"
	"github.com/xdung24/service-monitor/internal/database"
	"github.com/xdung24/service-monitor/internal/models"
	"github.com/xdung24/service-monitor/internal/scheduler"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "sm_session"

// Context keys injected by AuthRequired middleware.
const (
	ctxKeyUserDB   = "sm_user_db"
	ctxKeyUsername = "sm_username"
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

// AuthRequired is a Gin middleware that redirects unauthenticated requests to /login.
// It also opens the per-user database and injects it into the request context.
func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
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
		c.Set(ctxKeyUserDB, db)
		c.Set(ctxKeyUsername, username)
		c.Next()
	}
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
// Setup
// ---------------------------------------------------------------------------

// SetupPage renders the initial setup page.
func (h *Handler) SetupPage(c *gin.Context) {
	count, _ := h.users.Count()
	if count > 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	c.HTML(http.StatusOK, "setup.html", gin.H{"Error": ""})
}

// SetupSubmit handles the setup form, creating the first admin user.
func (h *Handler) SetupSubmit(c *gin.Context) {
	count, _ := h.users.Count()
	if count > 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	username := c.PostForm("username")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm_password")

	if username == "" || password == "" {
		c.HTML(http.StatusBadRequest, "setup.html", gin.H{"Error": "Username and password are required"})
		return
	}
	if password != confirm {
		c.HTML(http.StatusBadRequest, "setup.html", gin.H{"Error": "Passwords do not match"})
		return
	}
	if len(password) < 8 {
		c.HTML(http.StatusBadRequest, "setup.html", gin.H{"Error": "Password must be at least 8 characters"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "setup.html", gin.H{"Error": "Internal error"})
		return
	}

	if err := h.users.Create(username, string(hashed)); err != nil {
		c.HTML(http.StatusInternalServerError, "setup.html", gin.H{"Error": "Failed to create user"})
		return
	}

	// Initialize per-user database and start the scheduler for the new user.
	db, err := h.registry.Get(username)
	if err == nil {
		h.msched.StartForUser(username, db)
	}

	c.Redirect(http.StatusFound, "/login")
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// LoginPage renders the login form.
func (h *Handler) LoginPage(c *gin.Context) {
	count, _ := h.users.Count()
	if count == 0 {
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	c.HTML(http.StatusOK, "login.html", gin.H{"Error": ""})
}

// LoginSubmit validates credentials and sets a session cookie.
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

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Monitors":   monitors,
		"Sparklines": sparklines,
		"Tags":       tagMap,
		"Username":   h.username(c),
	})
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

	svg := fmt.Sprintf(
		`<svg width="%d" height="%d" viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" style="display:block;"><polyline points="%s" fill="none" stroke="#38bdf8" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/></svg>`,
		svgW, svgH, svgW, svgH, pts,
	)
	return template.HTML(svg)
}
