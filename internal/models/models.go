package models

import "time"

// MonitorType enumerates supported monitor types.
type MonitorType string

const (
	MonitorTypeHTTP MonitorType = "http"
	MonitorTypeTCP  MonitorType = "tcp"
	MonitorTypePing MonitorType = "ping"
	MonitorTypeDNS  MonitorType = "dns"
	MonitorTypePush MonitorType = "push"
)

// Monitor represents a monitored target.
type Monitor struct {
	ID              int64       `db:"id"`
	Name            string      `db:"name"`
	Type            MonitorType `db:"type"`
	URL             string      `db:"url"`
	IntervalSeconds int         `db:"interval_seconds"`
	TimeoutSeconds  int         `db:"timeout_seconds"`
	Active          bool        `db:"active"`
	Retries         int         `db:"retries"`
	DNSServer       string      `db:"dns_server"` // optional custom DNS server (host:port)
	CreatedAt       time.Time   `db:"created_at"`
	UpdatedAt       time.Time   `db:"updated_at"`

	// Computed fields (not stored in DB)
	LastStatus  *int    `db:"-"`
	LastLatency *int    `db:"-"`
	LastMessage *string `db:"-"`
	Uptime24h   float64 `db:"-"`
	Uptime30d   float64 `db:"-"`
}

// Heartbeat represents a single check result.
type Heartbeat struct {
	ID        int64     `db:"id"`
	MonitorID int64     `db:"monitor_id"`
	Status    int       `db:"status"` // 0=down, 1=up
	LatencyMs int       `db:"latency_ms"`
	Message   string    `db:"message"`
	CreatedAt time.Time `db:"created_at"`
}

// User represents a dashboard user.
type User struct {
	ID        int64     `db:"id"`
	Username  string    `db:"username"`
	Password  string    `db:"password"`
	CreatedAt time.Time `db:"created_at"`
}

// Notification holds notification provider configuration.
type Notification struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Type      string    `db:"type"`
	Config    string    `db:"config"` // JSON
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
}
