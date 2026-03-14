package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/config"
	"github.com/xdung24/service-monitor/internal/models"
	"github.com/xdung24/service-monitor/internal/scheduler"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "sm_session"

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db            *sql.DB
	sched         *scheduler.Scheduler
	cfg           *config.Config
	monitors      *models.MonitorStore
	heartbeat     *models.HeartbeatStore
	users         *models.UserStore
	notifications *models.NotificationStore
}

// New creates a Handler.
func New(db *sql.DB, sched *scheduler.Scheduler, cfg *config.Config) *Handler {
	return &Handler{
		db:            db,
		sched:         sched,
		cfg:           cfg,
		monitors:      models.NewMonitorStore(db),
		heartbeat:     models.NewHeartbeatStore(db),
		users:         models.NewUserStore(db),
		notifications: models.NewNotificationStore(db),
	}
}

// AuthRequired is a Gin middleware that redirects unauthenticated requests to /login.
func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || !h.validSession(cookie) {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) validSession(token string) bool {
	// Simple HMAC-signed username token; replace with a proper session store for production.
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
	monitors, err := h.monitors.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Error": err.Error()})
		return
	}

	now := time.Now()
	for _, m := range monitors {
		beats, _ := h.heartbeat.Latest(m.ID, 1)
		if len(beats) > 0 {
			m.LastStatus = &beats[0].Status
			m.LastLatency = &beats[0].LatencyMs
			m.LastMessage = &beats[0].Message
		}
		m.Uptime24h, _ = h.heartbeat.UptimePercent(m.ID, now.Add(-24*time.Hour))
		m.Uptime30d, _ = h.heartbeat.UptimePercent(m.ID, now.Add(-30*24*time.Hour))
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{"Monitors": monitors})
}
