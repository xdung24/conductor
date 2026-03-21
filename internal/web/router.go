package web

import (
	"database/sql"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/conductor/internal/config"
	"github.com/xdung24/conductor/internal/database"
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
func NewRouter(usersDB *sql.DB, registry *database.Registry, msched *scheduler.MultiScheduler, cfg *config.Config) http.Handler {
	r := gin.Default()

	// Templates are embedded in the binary at compile time; any parse error
	// crashes the process immediately at startup.
	r.SetHTMLTemplate(mustParseTemplates())

	h := handlers.New(usersDB, registry, msched, cfg)

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
	r.POST("/login", h.LoginSubmit)
	r.GET("/login/2fa", h.TwoFALoginPage)
	r.POST("/login/2fa", h.TwoFALoginSubmit)
	r.GET("/logout", h.Logout)

	// Self-registration (open or invite-token gated)
	r.GET("/register", h.RegisterPage)
	r.POST("/register", h.RegisterSubmit)

	// Push endpoint (unauthenticated — external services call this to signal UP).
	r.GET("/push/:token", h.MonitorPush)

	// Public status page (unauthenticated)
	r.GET("/status/:username/:slug", h.StatusPagePublic)
	r.GET("/status/:username/:slug/chart-data/:id", h.StatusPagePublicChartData)

	// Dashboard (protected)
	auth := r.Group("/")
	auth.Use(h.AuthRequired())
	{
		auth.GET("/", h.Dashboard)

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
		admin.POST("/admin/users/:username/delete", h.UserDelete)
		admin.POST("/admin/users/:username/role", h.UserSetAdmin)
		admin.POST("/admin/users/invite", h.InviteGenerate)
		admin.POST("/admin/users/invites/:token/delete", h.InviteRevoke)

		// Admin settings
		admin.GET("/admin/settings", h.SettingsPage)
		admin.POST("/admin/settings", h.SettingsUpdate)
	}

	return r
}
