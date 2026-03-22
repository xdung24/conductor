package web

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/config"
	"github.com/xdung24/conductor/internal/database"
	"github.com/xdung24/conductor/internal/mailer"
	"github.com/xdung24/conductor/internal/scheduler"
	"github.com/xdung24/conductor/internal/web/handlers"
)

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		"mapget": func(m map[string]string, key string) string {
			if m == nil {
				return ""
			}
			return m[key]
		},
		"sparkget": func(m map[int64]template.HTML, key int64) template.HTML {
			if m == nil {
				return ""
			}
			return m[key]
		},
		// slice returns a substring s[start:end], clamping end to len(s).
		"slice": func(s string, start, end int) string {
			if end > len(s) {
				end = len(s)
			}
			return s[start:end]
		},
		// typeLabel returns a human-readable label for a notification provider type.
		"typeLabel": func(t string) string {
			labels := map[string]string{
				"webhook":       "Webhook",
				"slack":         "Slack",
				"discord":       "Discord",
				"ntfy":          "ntfy",
				"telegram":      "Telegram",
				"email":         "Email (SMTP)",
				"mattermost":    "Mattermost",
				"rocketchat":    "Rocket.Chat",
				"dingding":      "DingTalk",
				"feishu":        "Feishu / Lark",
				"googlechat":    "Google Chat",
				"teams":         "MS Teams",
				"wecom":         "WeCom",
				"yzj":           "YZJ",
				"lunasea":       "LunaSea",
				"gotify":        "Gotify",
				"bark":          "Bark",
				"gorush":        "Gorush",
				"pushover":      "Pushover",
				"pushplus":      "PushPlus",
				"serverchan":    "ServerChan",
				"line":          "LINE Notify",
				"homeassistant": "Home Assistant",
				"pagerduty":     "PagerDuty",
				"matrix":        "Matrix",
				"signal":        "Signal",
				"waha":          "WAHA",
				"whapi":         "Whapi",
				"onesender":     "OneSender",
				"onebot":        "OneBot",
				"evolution":     "Evolution API",
				"sendgrid":      "SendGrid",
				"resend":        "Resend",
				"twilio":        "Twilio",
				"46elks":        "46elks",
				"brevo":         "Brevo SMS",
				"callmebot":     "CallMeBot",
				"cellsynt":      "Cellsynt",
				"freemobile":    "Free Mobile",
				"gtxmessaging":  "GTX Messaging",
				"octopush":      "Octopush",
				"promosms":      "PromoSMS",
				"serwersms":     "SerwerSMS",
				"sevenio":       "seven.io",
				"smsc":          "SMSC.ru",
				"smseagle":      "SMSEagle",
				"smsir":         "SMS.ir",
				"teltonika":     "Teltonika",
			}
			if l, ok := labels[t]; ok {
				return l
			}
			return t
		},
	}
}

func mustParseTemplates() *template.Template {
	tmpl, err := template.New("").Funcs(templateFuncMap()).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		panic(err)
	}
	return tmpl
}

// NewRouter builds and returns the Gin router.
func NewRouter(usersDB *sql.DB, registry *database.Registry, msched *scheduler.MultiScheduler, cfg *config.Config, m *mailer.Mailer) http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		entry := map[string]any{
			"time":       param.TimeStamp.UTC().Format(time.RFC3339),
			"status":     param.StatusCode,
			"latency_ms": float64(param.Latency.Microseconds()) / 1000.0,
			"client_ip":  param.ClientIP,
			"method":     param.Method,
			"path":       param.Path,
		}
		if param.ErrorMessage != "" {
			entry["error"] = param.ErrorMessage
		}
		b, _ := json.Marshal(entry)
		return string(b) + "\n"
	}))

	// Templates are embedded in the binary at compile time; any parse error
	// crashes the process immediately at startup.
	tmpl := mustParseTemplates()
	r.SetHTMLTemplate(tmpl)

	// Security headers applied to every response.
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	// Return an empty 404 for all unregistered routes — avoids leaking path
	// information and prevents Gin's default plain-text body.
	r.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	h := handlers.New(usersDB, registry, msched, cfg, tmpl, m)

	// Rate limiters for authentication endpoints.
	// 10 attempts / minute per IP on login; 5 / minute on registration.
	loginRL := handlers.RateLimiter(10, time.Minute)
	registerRL := handlers.RateLimiter(5, time.Minute)

	// Rate limiter for public (unauthenticated) endpoints.
	// StatusPagePublic has no DB-level caching; StatusPagePublicChartData has
	// a 60 s TTL cache but the first hit per key still queries the DB directly.
	// Both accept arbitrary user-supplied :username, so limiting per IP also
	// reduces username-enumeration and registry-open abuse.
	publicRL := handlers.RateLimiter(60, time.Minute)

	// Health endpoint (for Docker HEALTHCHECK)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Setup wizard removed — first-user registration is handled via a
	// time-limited token printed to the console on first startup.
	// Keep a redirect for anyone who bookmarked /setup.
	r.GET("/setup", func(c *gin.Context) { c.Redirect(http.StatusFound, "/login") })

	// Auth
	r.GET("/login", h.LoginPage)
	r.POST("/login", loginRL, h.LoginSubmit)
	r.GET("/login/2fa", h.TwoFALoginPage)
	r.POST("/login/2fa", loginRL, h.TwoFALoginSubmit)
	r.GET("/logout", h.Logout)

	// Self-registration (open or invite-token gated)
	r.GET("/register", h.RegisterPage)
	r.POST("/register", registerRL, h.RegisterSubmit)

	// Password reset (token-gated, admin-generated link)
	r.GET("/reset-password", h.ResetPasswordPage)
	r.POST("/reset-password", h.ResetPasswordSubmit)

	// Push endpoint (unauthenticated — external services call this to signal UP).
	r.GET("/push/:token", h.MonitorPush)

	// Public status page (unauthenticated)
	r.GET("/status/:username/:slug", publicRL, h.StatusPagePublic)
	r.GET("/status/:username/:slug/chart-data/:id", publicRL, h.StatusPagePublicChartData)

	// Public JSON summary endpoint (unauthenticated, no rate limit — cache-backed).
	// Enabled per-page via a UUID generated in the status page settings.
	r.GET("/summary/:uuid", h.StatusPagePublicSummary)

	// Dashboard (protected)
	auth := r.Group("/")
	auth.Use(h.AuthRequired())
	{
		auth.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/monitors") })
		auth.GET("/monitors", h.Dashboard)

		// Monitors
		auth.GET("/monitors/new", h.MonitorNew)
		auth.POST("/monitors", h.MonitorCreate)
		auth.GET("/monitors/:id", h.MonitorDetail)
		auth.GET("/monitors/:id/edit", h.MonitorEdit)
		auth.POST("/monitors/:id", h.MonitorUpdate)
		auth.POST("/monitors/:id/delete", h.MonitorDelete)
		auth.POST("/monitors/:id/pause", h.MonitorPause)
		auth.POST("/monitors/:id/resume", h.MonitorResume)
		auth.GET("/monitors/:id/export", h.MonitorExport)
		auth.GET("/monitors/:id/chart-data", h.MonitorChartData)
		auth.POST("/monitors/import", h.MonitorImport)

		// Notifications
		auth.GET("/notifications", h.NotificationList)
		auth.GET("/notifications/new", h.NotificationNew)
		auth.POST("/notifications", h.NotificationCreate)
		auth.GET("/notifications/:id/edit", h.NotificationEdit)
		auth.POST("/notifications/:id", h.NotificationUpdate)
		auth.POST("/notifications/:id/delete", h.NotificationDelete)
		auth.POST("/notifications/:id/test", h.NotificationTest)
		auth.GET("/notifications/logs", h.NotificationLogList)

		// Tags
		auth.GET("/tags", h.TagList)
		auth.GET("/tags/new", h.TagNew)
		auth.POST("/tags", h.TagCreate)
		auth.GET("/tags/:id/edit", h.TagEdit)
		auth.POST("/tags/:id", h.TagUpdate)
		auth.POST("/tags/:id/delete", h.TagDelete)

		// Status Pages
		auth.GET("/status-pages", h.StatusPageList)
		auth.GET("/status-pages/new", h.StatusPageNew)
		auth.POST("/status-pages", h.StatusPageCreate)
		auth.GET("/status-pages/:id/edit", h.StatusPageEdit)
		auth.POST("/status-pages/:id", h.StatusPageUpdate)
		auth.POST("/status-pages/:id/delete", h.StatusPageDelete)

		// Maintenance
		auth.GET("/maintenance", h.MaintenanceList)
		auth.GET("/maintenance/new", h.MaintenanceNew)
		auth.POST("/maintenance", h.MaintenanceCreate)
		auth.GET("/maintenance/:id/edit", h.MaintenanceEdit)
		auth.POST("/maintenance/:id", h.MaintenanceUpdate)
		auth.POST("/maintenance/:id/delete", h.MaintenanceDelete)

		// Docker Hosts
		auth.GET("/docker-hosts", h.DockerHostList)
		auth.GET("/docker-hosts/new", h.DockerHostNew)
		auth.POST("/docker-hosts", h.DockerHostCreate)
		auth.GET("/docker-hosts/:id/edit", h.DockerHostEdit)
		auth.POST("/docker-hosts/:id", h.DockerHostUpdate)
		auth.POST("/docker-hosts/:id/delete", h.DockerHostDelete)

		// Proxies
		auth.GET("/proxies", h.ProxyList)
		auth.GET("/proxies/new", h.ProxyNew)
		auth.POST("/proxies", h.ProxyCreate)
		auth.GET("/proxies/:id/edit", h.ProxyEdit)
		auth.POST("/proxies/:id", h.ProxyUpdate)
		auth.POST("/proxies/:id/delete", h.ProxyDelete)

		// API Keys
		auth.GET("/api-keys", h.APIKeyList)
		auth.POST("/api-keys", h.APIKeyCreate)
		auth.POST("/api-keys/:id/delete", h.APIKeyDelete)

		// Account — 2FA
		auth.GET("/account/2fa", h.TwoFAPage)
		auth.POST("/account/2fa/setup", h.TwoFASetupPage)
		auth.POST("/account/2fa/verify", h.TwoFAVerify)
		auth.POST("/account/2fa/disable", h.TwoFADisable)

		// Documentation
		auth.GET("/docs", h.DocsPage)

		// Settings (per-user, e.g. theme)
		auth.POST("/settings/theme", h.ThemeToggle)
	}

	// Admin-only routes (requires both authentication and admin role).
	admin := r.Group("/")
	admin.Use(h.AuthRequired(), h.AdminRequired())
	{
		// User management
		admin.GET("/admin/users", h.UserList)
		admin.GET("/admin/users/new", h.UserNew)
		admin.POST("/admin/users", h.UserCreate)
		admin.GET("/admin/users/:username/password", h.UserPasswordPage)
		admin.POST("/admin/users/:username/password", h.UserChangePassword)
		admin.POST("/admin/users/:username/reset-link", h.UserGenerateResetLink)
		admin.POST("/admin/users/:username/toggle-disabled", h.UserToggleDisabled)
		admin.POST("/admin/users/:username/remove-2fa", h.UserRemove2FA)
		admin.POST("/admin/users/invite", h.InviteGenerate)
		admin.POST("/admin/users/invites/:token/delete", h.InviteRevoke)

		// Admin settings
		admin.GET("/admin/settings", h.SettingsPage)
		admin.POST("/admin/settings", h.SettingsUpdate)
	}

	return r
}
