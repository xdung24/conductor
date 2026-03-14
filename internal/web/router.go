package web

import (
	"database/sql"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xdung24/service-monitor/internal/config"
	"github.com/xdung24/service-monitor/internal/scheduler"
	"github.com/xdung24/service-monitor/internal/web/handlers"
)

// NewRouter builds and returns the Gin router.
func NewRouter(db *sql.DB, sched *scheduler.Scheduler, cfg *config.Config) http.Handler {
	r := gin.Default()

	// Register template helper functions before loading templates.
	r.SetFuncMap(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		// index is already built-in to html/template, but Gin's FuncMap needs it
		// explicitly when using maps with string keys in templates.
		"mapget": func(m map[string]string, key string) string {
			if m == nil {
				return ""
			}
			return m[key]
		},
	})

	r.LoadHTMLGlob("internal/web/templates/*.html")

	h := handlers.New(db, sched, cfg)

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

		// Notifications
		auth.GET("/notifications", h.NotificationList)
		auth.GET("/notifications/new", h.NotificationNew)
		auth.POST("/notifications", h.NotificationCreate)
		auth.GET("/notifications/:id/edit", h.NotificationEdit)
		auth.POST("/notifications/:id", h.NotificationUpdate)
		auth.POST("/notifications/:id/delete", h.NotificationDelete)
		auth.POST("/notifications/:id/test", h.NotificationTest)
	}

	return r
}
