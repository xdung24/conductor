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
	MonitorTypeSMTP MonitorType = "smtp"
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
	DNSServer       string      `db:"dns_server"`      // optional custom DNS resolver (host[:port])
	DNSRecordType   string      `db:"dns_record_type"` // A, AAAA, CNAME, MX, NS, TXT, PTR (DNS type only)
	DNSExpected     string      `db:"dns_expected"`    // optional expected value to match in answer

	// HTTP extended options
	HTTPAcceptedStatuses string `db:"http_accepted_statuses"` // comma-separated accepted status codes; empty = 2xx/3xx
	HTTPIgnoreTLS        bool   `db:"http_ignore_tls"`        // skip TLS certificate verification
	HTTPMethod           string `db:"http_method"`            // HTTP method (GET, POST, HEAD, …); default GET
	HTTPKeyword          string `db:"http_keyword"`           // response body must contain this string (if non-empty)
	HTTPKeywordInvert    bool   `db:"http_keyword_invert"`    // invert: body must NOT contain keyword
	HTTPUsername         string `db:"http_username"`          // HTTP basic-auth username
	HTTPPassword         string `db:"http_password"`          // HTTP basic-auth password
	HTTPBearerToken      string `db:"http_bearer_token"`      // bearer token (takes priority over basic auth)
	HTTPMaxRedirects     int    `db:"http_max_redirects"`     // 0 = no follow; positive = limit; default 10

	// Push/Heartbeat monitor
	PushToken string `db:"push_token"` // random token for push endpoint (/push/:token)

	// Response assertion fields (HTTP only)
	HTTPHeaderName    string `db:"http_header_name"`    // response header to check; empty = skip
	HTTPHeaderValue   string `db:"http_header_value"`   // expected value; empty = presence-only check
	HTTPBodyType      string `db:"http_body_type"`      // "": any, "json", "xml", "text", "binary"
	HTTPJsonPath      string `db:"http_json_path"`      // JSONPath expression e.g. $.status
	HTTPJsonExpected  string `db:"http_json_expected"`  // expected value; empty = just check path exists
	HTTPXPath         string `db:"http_xpath"`          // XPath expression e.g. //status
	HTTPXPathExpected string `db:"http_xpath_expected"` // expected value; empty = just check node exists

	// Custom request options (HTTP only)
	HTTPRequestHeaders string `db:"http_request_headers"` // Key: Value lines
	HTTPRequestBody    string `db:"http_request_body"`    // raw body for POST/PUT/PATCH

	// SMTP monitor fields
	SMTPUseTLS    bool   `db:"smtp_use_tls"`    // use implicit TLS / SMTPS (port 465)
	SMTPIgnoreTLS bool   `db:"smtp_ignore_tls"` // skip TLS certificate verification
	SMTPUsername  string `db:"smtp_username"`   // optional AUTH PLAIN username
	SMTPPassword  string `db:"smtp_password"`   // optional AUTH PLAIN password

	// Notification trigger settings
	NotifyOnFailure bool `db:"notify_on_failure"` // send notification when check result is DOWN
	NotifyOnSuccess bool `db:"notify_on_success"` // send notification when check result is UP
	NotifyBodyChars int  `db:"notify_body_chars"` // include up to N chars of HTTP response body in notification; 0 = disabled

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

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

// NotificationLog records a single notification delivery attempt.
type NotificationLog struct {
	ID               int64     `db:"id"`
	MonitorID        *int64    `db:"monitor_id"`      // nullable (monitor may be deleted)
	NotificationID   *int64    `db:"notification_id"` // nullable
	MonitorName      string    `db:"monitor_name"`
	NotificationName string    `db:"notification_name"`
	EventStatus      int       `db:"event_status"` // 0=down, 1=up
	Success          bool      `db:"success"`
	Error            string    `db:"error"`
	CreatedAt        time.Time `db:"created_at"`
}

// StatusText returns "UP" or "DOWN" for the logged event.
func (l *NotificationLog) StatusText() string {
	if l.EventStatus == 1 {
		return "UP"
	}
	return "DOWN"
}
