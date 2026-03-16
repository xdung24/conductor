package web

import (
	"database/sql"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/config"
	"github.com/xdung24/service-monitor/internal/database"
	"github.com/xdung24/service-monitor/internal/scheduler"
	"github.com/xdung24/service-monitor/internal/web/handlers"
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

	// Setup wizard (only accessible when no users exist)
	r.GET("/setup", h.SetupPage)
	r.POST("/setup", h.SetupSubmit)

	// Auth
	r.GET("/login", h.LoginPage)
	r.POST("/login", h.LoginSubmit)
	r.GET("/logout", h.Logout)

	// Push endpoint (unauthenticated — external services call this to signal UP).
	r.GET("/push/:token", h.MonitorPush)

	// Public status page (unauthenticated)
	r.GET("/status/:username/:slug", h.StatusPagePublic)

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

		// User management
		auth.GET("/admin/users", h.UserList)
		auth.GET("/admin/users/new", h.UserNew)
		auth.POST("/admin/users", h.UserCreate)
		auth.GET("/admin/users/:username/password", h.UserPasswordPage)
		auth.POST("/admin/users/:username/password", h.UserChangePassword)
		auth.POST("/admin/users/:username/delete", h.UserDelete)

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

		// Settings
		auth.POST("/settings/theme", h.ThemeToggle)
	}

	return r
}
