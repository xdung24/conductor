package models

import (
	"database/sql"
	"fmt"
	"time"
)

// MonitorStore handles all monitor-related DB operations.
type MonitorStore struct {
	db *sql.DB
}

// NewMonitorStore creates a new MonitorStore.
func NewMonitorStore(db *sql.DB) *MonitorStore {
	return &MonitorStore{db: db}
}

// List returns all monitors with their latest heartbeat status.
func (s *MonitorStore) List() ([]*Monitor, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, url, interval_seconds, timeout_seconds, active, retries,
		       dns_server, dns_record_type, dns_expected,
		       http_accepted_statuses, http_ignore_tls, http_method, http_keyword, http_keyword_invert,
		       http_username, http_password, http_bearer_token, http_max_redirects,
		       push_token,
		       http_header_name, http_header_value, http_body_type,
		       http_json_path, http_json_expected, http_xpath, http_xpath_expected,
		       smtp_use_tls, smtp_ignore_tls, smtp_username, smtp_password,
		       notify_on_failure, notify_on_success, notify_body_chars,
		       http_request_headers, http_request_body,
		       created_at, updated_at
		FROM monitors ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*Monitor
	for rows.Next() {
		m := &Monitor{}
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.URL, &m.IntervalSeconds,
			&m.TimeoutSeconds, &m.Active, &m.Retries, &m.DNSServer,
			&m.DNSRecordType, &m.DNSExpected,
			&m.HTTPAcceptedStatuses, &m.HTTPIgnoreTLS, &m.HTTPMethod, &m.HTTPKeyword, &m.HTTPKeywordInvert,
			&m.HTTPUsername, &m.HTTPPassword, &m.HTTPBearerToken, &m.HTTPMaxRedirects,
			&m.PushToken,
			&m.HTTPHeaderName, &m.HTTPHeaderValue, &m.HTTPBodyType,
			&m.HTTPJsonPath, &m.HTTPJsonExpected, &m.HTTPXPath, &m.HTTPXPathExpected,
			&m.SMTPUseTLS, &m.SMTPIgnoreTLS, &m.SMTPUsername, &m.SMTPPassword,
			&m.NotifyOnFailure, &m.NotifyOnSuccess, &m.NotifyBodyChars,
			&m.HTTPRequestHeaders, &m.HTTPRequestBody,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}

// Get returns a single monitor by ID.
func (s *MonitorStore) Get(id int64) (*Monitor, error) {
	m := &Monitor{}
	err := s.db.QueryRow(`
		SELECT id, name, type, url, interval_seconds, timeout_seconds, active, retries,
		       dns_server, dns_record_type, dns_expected,
		       http_accepted_statuses, http_ignore_tls, http_method, http_keyword, http_keyword_invert,
		       http_username, http_password, http_bearer_token, http_max_redirects,
		       push_token,
		       http_header_name, http_header_value, http_body_type,
		       http_json_path, http_json_expected, http_xpath, http_xpath_expected,
		       smtp_use_tls, smtp_ignore_tls, smtp_username, smtp_password,
		       notify_on_failure, notify_on_success, notify_body_chars,
		       http_request_headers, http_request_body,
		       created_at, updated_at
		FROM monitors WHERE id = ?
	`, id).Scan(&m.ID, &m.Name, &m.Type, &m.URL, &m.IntervalSeconds,
		&m.TimeoutSeconds, &m.Active, &m.Retries, &m.DNSServer,
		&m.DNSRecordType, &m.DNSExpected,
		&m.HTTPAcceptedStatuses, &m.HTTPIgnoreTLS, &m.HTTPMethod, &m.HTTPKeyword, &m.HTTPKeywordInvert,
		&m.HTTPUsername, &m.HTTPPassword, &m.HTTPBearerToken, &m.HTTPMaxRedirects,
		&m.PushToken,
		&m.HTTPHeaderName, &m.HTTPHeaderValue, &m.HTTPBodyType,
		&m.HTTPJsonPath, &m.HTTPJsonExpected, &m.HTTPXPath, &m.HTTPXPathExpected,
		&m.SMTPUseTLS, &m.SMTPIgnoreTLS, &m.SMTPUsername, &m.SMTPPassword,
		&m.NotifyOnFailure, &m.NotifyOnSuccess, &m.NotifyBodyChars,
		&m.HTTPRequestHeaders, &m.HTTPRequestBody,
		&m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// Create inserts a new monitor and returns its ID.
func (s *MonitorStore) Create(m *Monitor) (int64, error) {
	now := time.Now()
	res, err := s.db.Exec(`
		INSERT INTO monitors (name, type, url, interval_seconds, timeout_seconds, active, retries,
		                      dns_server, dns_record_type, dns_expected,
		                      http_accepted_statuses, http_ignore_tls, http_method, http_keyword, http_keyword_invert,
		                      http_username, http_password, http_bearer_token, http_max_redirects,
		                      push_token,
		                      http_header_name, http_header_value, http_body_type,
		                      http_json_path, http_json_expected, http_xpath, http_xpath_expected,
		                      smtp_use_tls, smtp_ignore_tls, smtp_username, smtp_password,
		                      notify_on_failure, notify_on_success, notify_body_chars,
		                      http_request_headers, http_request_body,
		                      created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.Name, m.Type, m.URL, m.IntervalSeconds, m.TimeoutSeconds, m.Active, m.Retries,
		m.DNSServer, m.DNSRecordType, m.DNSExpected,
		m.HTTPAcceptedStatuses, m.HTTPIgnoreTLS, m.HTTPMethod, m.HTTPKeyword, m.HTTPKeywordInvert,
		m.HTTPUsername, m.HTTPPassword, m.HTTPBearerToken, m.HTTPMaxRedirects,
		m.PushToken,
		m.HTTPHeaderName, m.HTTPHeaderValue, m.HTTPBodyType,
		m.HTTPJsonPath, m.HTTPJsonExpected, m.HTTPXPath, m.HTTPXPathExpected,
		m.SMTPUseTLS, m.SMTPIgnoreTLS, m.SMTPUsername, m.SMTPPassword,
		m.NotifyOnFailure, m.NotifyOnSuccess, m.NotifyBodyChars,
		m.HTTPRequestHeaders, m.HTTPRequestBody,
		now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing monitor.
func (s *MonitorStore) Update(m *Monitor) error {
	_, err := s.db.Exec(`
		UPDATE monitors SET name=?, type=?, url=?, interval_seconds=?, timeout_seconds=?,
		active=?, retries=?, dns_server=?, dns_record_type=?, dns_expected=?,
		http_accepted_statuses=?, http_ignore_tls=?, http_method=?, http_keyword=?, http_keyword_invert=?,
		http_username=?, http_password=?, http_bearer_token=?, http_max_redirects=?,
		push_token=?,
		http_header_name=?, http_header_value=?, http_body_type=?,
		http_json_path=?, http_json_expected=?, http_xpath=?, http_xpath_expected=?,
		smtp_use_tls=?, smtp_ignore_tls=?, smtp_username=?, smtp_password=?,
		notify_on_failure=?, notify_on_success=?, notify_body_chars=?,
		http_request_headers=?, http_request_body=?,
		updated_at=? WHERE id=?
	`, m.Name, m.Type, m.URL, m.IntervalSeconds, m.TimeoutSeconds, m.Active, m.Retries,
		m.DNSServer, m.DNSRecordType, m.DNSExpected,
		m.HTTPAcceptedStatuses, m.HTTPIgnoreTLS, m.HTTPMethod, m.HTTPKeyword, m.HTTPKeywordInvert,
		m.HTTPUsername, m.HTTPPassword, m.HTTPBearerToken, m.HTTPMaxRedirects,
		m.PushToken,
		m.HTTPHeaderName, m.HTTPHeaderValue, m.HTTPBodyType,
		m.HTTPJsonPath, m.HTTPJsonExpected, m.HTTPXPath, m.HTTPXPathExpected,
		m.SMTPUseTLS, m.SMTPIgnoreTLS, m.SMTPUsername, m.SMTPPassword,
		m.NotifyOnFailure, m.NotifyOnSuccess, m.NotifyBodyChars,
		m.HTTPRequestHeaders, m.HTTPRequestBody,
		time.Now(), m.ID)
	return err
}

// Delete removes a monitor (cascades to heartbeats).
func (s *MonitorStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM monitors WHERE id = ?`, id)
	return err
}

// SetActive pauses or resumes a monitor.
func (s *MonitorStore) SetActive(id int64, active bool) error {
	_, err := s.db.Exec(`UPDATE monitors SET active=?, updated_at=? WHERE id=?`, active, time.Now(), id)
	return err
}

// HeartbeatStore handles heartbeat DB operations.
type HeartbeatStore struct {
	db *sql.DB
}

// NewHeartbeatStore creates a new HeartbeatStore.
func NewHeartbeatStore(db *sql.DB) *HeartbeatStore {
	return &HeartbeatStore{db: db}
}

// Insert saves a heartbeat result.
func (s *HeartbeatStore) Insert(h *Heartbeat) error {
	_, err := s.db.Exec(`
		INSERT INTO heartbeats (monitor_id, status, latency_ms, message, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, h.MonitorID, h.Status, h.LatencyMs, h.Message, h.CreatedAt)
	return err
}

// Latest returns the most recent N heartbeats for a monitor.
func (s *HeartbeatStore) Latest(monitorID int64, limit int) ([]*Heartbeat, error) {
	rows, err := s.db.Query(`
		SELECT id, monitor_id, status, latency_ms, message, created_at
		FROM heartbeats WHERE monitor_id = ?
		ORDER BY created_at DESC LIMIT ?
	`, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var beats []*Heartbeat
	for rows.Next() {
		h := &Heartbeat{}
		if err := rows.Scan(&h.ID, &h.MonitorID, &h.Status, &h.LatencyMs, &h.Message, &h.CreatedAt); err != nil {
			return nil, err
		}
		beats = append(beats, h)
	}
	return beats, rows.Err()
}

// UptimePercent returns uptime % for a monitor over the given duration.
func (s *HeartbeatStore) UptimePercent(monitorID int64, since time.Time) (float64, error) {
	var total, up int
	err := s.db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN status=1 THEN 1 ELSE 0 END)
		FROM heartbeats WHERE monitor_id=? AND created_at >= ?
	`, monitorID, since).Scan(&total, &up)
	if err != nil || total == 0 {
		return 0, err
	}
	return float64(up) / float64(total) * 100, nil
}

// UserStore handles user DB operations.
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a new UserStore.
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

// Count returns number of users.
func (s *UserStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// GetByUsername returns a user by username.
func (s *UserStore) GetByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(`
		SELECT id, username, password, created_at FROM users WHERE username=?
	`, username).Scan(&u.ID, &u.Username, &u.Password, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// Create inserts a new user.
func (s *UserStore) Create(username, hashedPassword string) error {
	_, err := s.db.Exec(`INSERT INTO users (username, password) VALUES (?, ?)`, username, hashedPassword)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// MonitorStore — state tracking for notifications
// ---------------------------------------------------------------------------

// UpdateLastStatus stores the last observed status and the last status for
// which a notification was fired.
func (s *MonitorStore) UpdateLastStatus(id int64, status int) error {
	_, err := s.db.Exec(
		`UPDATE monitors SET last_status=? WHERE id=?`,
		status, id,
	)
	return err
}

// UpdateLastNotifiedStatus records that a notification was sent for the given status.
func (s *MonitorStore) UpdateLastNotifiedStatus(id int64, status int) error {
	_, err := s.db.Exec(
		`UPDATE monitors SET last_notified_status=? WHERE id=?`,
		status, id,
	)
	return err
}

// GetLastStatuses returns (lastStatus, lastNotifiedStatus) for a monitor.
// Both values are nil-able (NULL before first check / first notification).
func (s *MonitorStore) GetLastStatuses(id int64) (lastStatus, lastNotified *int, err error) {
	var ls, ln sql.NullInt64
	err = s.db.QueryRow(
		`SELECT last_status, last_notified_status FROM monitors WHERE id=?`, id,
	).Scan(&ls, &ln)
	if err != nil {
		return nil, nil, err
	}
	if ls.Valid {
		v := int(ls.Int64)
		lastStatus = &v
	}
	if ln.Valid {
		v := int(ln.Int64)
		lastNotified = &v
	}
	return lastStatus, lastNotified, nil
}

// GetByPushToken returns the monitor with the given push token, or nil if not found.
func (s *MonitorStore) GetByPushToken(token string) (*Monitor, error) {
	m := &Monitor{}
	err := s.db.QueryRow(`
		SELECT id, name, type, url, interval_seconds, timeout_seconds, active, retries,
		       dns_server, dns_record_type, dns_expected,
		       http_accepted_statuses, http_ignore_tls, http_method, http_keyword, http_keyword_invert,
		       http_username, http_password, http_bearer_token, http_max_redirects,
		       push_token,
		       http_header_name, http_header_value, http_body_type,
		       http_json_path, http_json_expected, http_xpath, http_xpath_expected,
		       smtp_use_tls, smtp_ignore_tls, smtp_username, smtp_password,
		       notify_on_failure, notify_on_success, notify_body_chars,
		       http_request_headers, http_request_body,
		       created_at, updated_at
		FROM monitors WHERE push_token = ? AND push_token != ''
	`, token).Scan(&m.ID, &m.Name, &m.Type, &m.URL, &m.IntervalSeconds,
		&m.TimeoutSeconds, &m.Active, &m.Retries, &m.DNSServer,
		&m.DNSRecordType, &m.DNSExpected,
		&m.HTTPAcceptedStatuses, &m.HTTPIgnoreTLS, &m.HTTPMethod, &m.HTTPKeyword, &m.HTTPKeywordInvert,
		&m.HTTPUsername, &m.HTTPPassword, &m.HTTPBearerToken, &m.HTTPMaxRedirects,
		&m.PushToken,
		&m.HTTPHeaderName, &m.HTTPHeaderValue, &m.HTTPBodyType,
		&m.HTTPJsonPath, &m.HTTPJsonExpected, &m.HTTPXPath, &m.HTTPXPathExpected,
		&m.SMTPUseTLS, &m.SMTPIgnoreTLS, &m.SMTPUsername, &m.SMTPPassword,
		&m.NotifyOnFailure, &m.NotifyOnSuccess, &m.NotifyBodyChars,
		&m.HTTPRequestHeaders, &m.HTTPRequestBody,
		&m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// ---------------------------------------------------------------------------
// NotificationStore
// ---------------------------------------------------------------------------

// NotificationStore handles notification provider DB operations.
type NotificationStore struct {
	db *sql.DB
}

// NewNotificationStore creates a new NotificationStore.
func NewNotificationStore(db *sql.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

// List returns all notification providers.
func (s *NotificationStore) List() ([]*Notification, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, config, active, created_at FROM notifications ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.Config, &n.Active, &n.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// Get returns a single notification provider by ID.
func (s *NotificationStore) Get(id int64) (*Notification, error) {
	n := &Notification{}
	err := s.db.QueryRow(`
		SELECT id, name, type, config, active, created_at FROM notifications WHERE id=?
	`, id).Scan(&n.ID, &n.Name, &n.Type, &n.Config, &n.Active, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

// Create inserts a new notification provider and returns its ID.
func (s *NotificationStore) Create(n *Notification) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO notifications (name, type, config, active) VALUES (?, ?, ?, ?)
	`, n.Name, n.Type, n.Config, n.Active)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing notification provider.
func (s *NotificationStore) Update(n *Notification) error {
	_, err := s.db.Exec(`
		UPDATE notifications SET name=?, type=?, config=?, active=? WHERE id=?
	`, n.Name, n.Type, n.Config, n.Active, n.ID)
	return err
}

// Delete removes a notification provider.
func (s *NotificationStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notifications WHERE id=?`, id)
	return err
}

// ListForMonitor returns all active notification providers linked to a monitor.
func (s *NotificationStore) ListForMonitor(monitorID int64) ([]*Notification, error) {
	rows, err := s.db.Query(`
		SELECT n.id, n.name, n.type, n.config, n.active, n.created_at
		FROM notifications n
		JOIN monitor_notifications mn ON mn.notification_id = n.id
		WHERE mn.monitor_id=? AND n.active=1
	`, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.Config, &n.Active, &n.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// LinkMonitor attaches a notification to a monitor (idempotent).
func (s *NotificationStore) LinkMonitor(monitorID, notificationID int64) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO monitor_notifications (monitor_id, notification_id) VALUES (?, ?)
	`, monitorID, notificationID)
	return err
}

// UnlinkMonitor detaches a notification from a monitor.
func (s *NotificationStore) UnlinkMonitor(monitorID, notificationID int64) error {
	_, err := s.db.Exec(`
		DELETE FROM monitor_notifications WHERE monitor_id=? AND notification_id=?
	`, monitorID, notificationID)
	return err
}

// ReplaceMonitorLinks replaces all notification links for a monitor atomically.
func (s *NotificationStore) ReplaceMonitorLinks(monitorID int64, notificationIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM monitor_notifications WHERE monitor_id=?`, monitorID); err != nil {
		return err
	}
	for _, nid := range notificationIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO monitor_notifications (monitor_id, notification_id) VALUES (?, ?)`,
			monitorID, nid,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// NotificationLogStore
// ---------------------------------------------------------------------------

// NotificationLogStore handles notification delivery log operations.
type NotificationLogStore struct {
	db *sql.DB
}

// NewNotificationLogStore creates a new NotificationLogStore.
func NewNotificationLogStore(db *sql.DB) *NotificationLogStore {
	return &NotificationLogStore{db: db}
}

// Insert records a notification delivery attempt.
func (s *NotificationLogStore) Insert(l *NotificationLog) error {
	_, err := s.db.Exec(`
		INSERT INTO notification_logs
			(monitor_id, notification_id, monitor_name, notification_name, event_status, success, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, l.MonitorID, l.NotificationID, l.MonitorName, l.NotificationName,
		l.EventStatus, l.Success, l.Error, l.CreatedAt)
	return err
}

// List returns the most recent notification log entries (newest first).
func (s *NotificationLogStore) List(limit int) ([]*NotificationLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`
		SELECT id, monitor_id, notification_id, monitor_name, notification_name,
		       event_status, success, error, created_at
		FROM notification_logs
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*NotificationLog
	for rows.Next() {
		l := &NotificationLog{}
		if err := rows.Scan(&l.ID, &l.MonitorID, &l.NotificationID, &l.MonitorName,
			&l.NotificationName, &l.EventStatus, &l.Success, &l.Error, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ListForMonitor returns the most recent log entries for a specific monitor.
func (s *NotificationLogStore) ListForMonitor(monitorID int64, limit int) ([]*NotificationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, monitor_id, notification_id, monitor_name, notification_name,
		       event_status, success, error, created_at
		FROM notification_logs
		WHERE monitor_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*NotificationLog
	for rows.Next() {
		l := &NotificationLog{}
		if err := rows.Scan(&l.ID, &l.MonitorID, &l.NotificationID, &l.MonitorName,
			&l.NotificationName, &l.EventStatus, &l.Success, &l.Error, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
