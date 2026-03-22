package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/xdung24/conductor/internal/models"
	"github.com/xdung24/conductor/internal/monitor"
	"github.com/xdung24/conductor/internal/notifier"
)

// Scheduler manages periodic monitor checks.
type Scheduler struct {
	db             *sql.DB
	monitors       *models.MonitorStore
	heartbeat      *models.HeartbeatStore
	notifications  *models.NotificationStore
	notifLogs      *models.NotificationLogStore
	maintenance    *models.MaintenanceStore
	downtimeEvents *models.DowntimeEventStore
	jobs           map[int64]*job
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	// Per-monitor network resource caches (keyed by monitor ID).
	// Created at Schedule time, closed and evicted at Unschedule/Stop time.
	caches   map[int64]monitor.Cache
	cachesMu sync.Mutex
}

type job struct {
	monitorID int64
	ticker    *time.Ticker
	stop      chan struct{}
}

// New creates a new Scheduler.
func New(db *sql.DB) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		db:             db,
		monitors:       models.NewMonitorStore(db),
		heartbeat:      models.NewHeartbeatStore(db),
		notifications:  models.NewNotificationStore(db),
		notifLogs:      models.NewNotificationLogStore(db),
		maintenance:    models.NewMaintenanceStore(db),
		downtimeEvents: models.NewDowntimeEventStore(db),
		jobs:           make(map[int64]*job),
		caches:         make(map[int64]monitor.Cache),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start loads all active monitors and begins scheduling them.
func (s *Scheduler) Start() {
	monitors, err := s.monitors.List()
	if err != nil {
		slog.Error("scheduler: failed to load monitors", "error", err)
		return
	}

	for _, m := range monitors {
		if m.Active {
			s.Schedule(m)
		}
	}
	slog.Info("scheduler: started", "monitors", len(s.jobs))
}

// Stop cancels all running jobs and releases cached network resources.
func (s *Scheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	for _, j := range s.jobs {
		close(j.stop)
	}
	s.mu.Unlock()
	s.cachesMu.Lock()
	for _, cache := range s.caches {
		if cache.DBConn != nil {
			cache.DBConn.Close() //nolint:errcheck
		}
	}
	s.cachesMu.Unlock()
}

// Schedule adds or replaces the schedule for a single monitor.
func (s *Scheduler) Schedule(m *models.Monitor) {
	s.Unschedule(m.ID)

	if !m.Active {
		return
	}

	// Push monitors receive heartbeats via the /push/:token endpoint; they are
	// not polled by the scheduler.
	if m.Type == models.MonitorTypePush {
		return
	}

	interval := time.Duration(m.IntervalSeconds) * time.Second
	j := &job{
		monitorID: m.ID,
		ticker:    time.NewTicker(interval),
		stop:      make(chan struct{}),
	}

	s.mu.Lock()
	s.jobs[m.ID] = j
	s.mu.Unlock()

	// Build and cache network resources for this monitor.
	var cache monitor.Cache
	switch m.Type {
	case models.MonitorTypeHTTP, models.MonitorTypeRabbitMQ:
		proxyURL := ""
		if m.Type == models.MonitorTypeHTTP && m.ProxyID > 0 && monitor.ProxyLookup != nil {
			proxyURL = monitor.ProxyLookup(s.db, m.ProxyID)
		}
		cache.HTTPClient = monitor.NewHTTPClient(m, proxyURL)
	case models.MonitorTypeMySQL, models.MonitorTypePostgres, models.MonitorTypeMSSQL:
		cache.DBConn = monitor.NewDBConn(m)
	}
	s.cachesMu.Lock()
	s.caches[m.ID] = cache
	s.cachesMu.Unlock()

	// Run immediately on first schedule
	go s.runCheck(m)

	go func() {
		for {
			select {
			case <-j.ticker.C:
				// Re-fetch the monitor in case it was updated
				latest, err := s.monitors.Get(m.ID)
				if err != nil || latest == nil {
					s.Unschedule(m.ID)
					return
				}
				// Skip check if within an active maintenance window.
				if inMaint, _ := s.maintenance.IsInMaintenance(latest.ID, time.Now().UTC()); inMaint {
					slog.Info("monitor skipped: maintenance window active", "monitor_id", latest.ID, "monitor_name", latest.Name)
					continue
				}
				go s.runCheck(latest)
			case <-j.stop:
				j.ticker.Stop()
				return
			case <-s.ctx.Done():
				j.ticker.Stop()
				return
			}
		}
	}()
}

// Unschedule stops the job for a given monitor ID and releases its cached resources.
func (s *Scheduler) Unschedule(id int64) {
	s.mu.Lock()
	if j, ok := s.jobs[id]; ok {
		close(j.stop)
		delete(s.jobs, id)
	}
	s.mu.Unlock()
	s.cachesMu.Lock()
	if cache, ok := s.caches[id]; ok {
		if cache.DBConn != nil {
			cache.DBConn.Close() //nolint:errcheck
		}
		delete(s.caches, id)
	}
	s.cachesMu.Unlock()
}

func (s *Scheduler) runCheck(m *models.Monitor) {
	s.cachesMu.Lock()
	cache := s.caches[m.ID]
	s.cachesMu.Unlock()

	result := monitor.Run(s.ctx, s.db, cache, m)

	now := time.Now().UTC()

	// Get previous status for transition detection before recording new result.
	prevStatus, _, _ := s.monitors.GetLastStatuses(m.ID)

	h := &models.Heartbeat{
		MonitorID: m.ID,
		Status:    result.Status,
		LatencyMs: result.LatencyMs,
		Message:   result.Message,
		CreatedAt: now,
	}

	if err := s.heartbeat.Insert(h); err != nil {
		slog.Error("scheduler: failed to save heartbeat", "monitor_id", m.ID, "error", err)
	}

	statusText := "UP"
	if result.Status == 0 {
		statusText = "DOWN"
	}
	slog.Info("monitor check", "monitor_id", m.ID, "monitor_name", m.Name, "status", statusText, "latency_ms", result.LatencyMs, "message", result.Message)

	// Track downtime events on state transitions.
	if result.Status == 0 && (prevStatus == nil || *prevStatus != 0) {
		if err := s.downtimeEvents.OpenIncident(m.ID, now); err != nil {
			slog.Error("scheduler: open incident error", "monitor_id", m.ID, "error", err)
		}
	} else if result.Status == 1 && prevStatus != nil && *prevStatus == 0 {
		if err := s.downtimeEvents.CloseIncident(m.ID, now); err != nil {
			slog.Error("scheduler: close incident error", "monitor_id", m.ID, "error", err)
		}
	}

	// State-change detection — only notify when status flips.
	s.maybeNotify(m, result)

	// Persist the last status for the next comparison.
	if err := s.monitors.UpdateLastStatus(m.ID, result.Status); err != nil {
		slog.Error("scheduler: failed to update last_status", "monitor_id", m.ID, "error", err)
	}
}

// maybeNotify fires notifications only when the monitor changes state.
func (s *Scheduler) maybeNotify(m *models.Monitor, result monitor.Result) {
	_, lastNotified, err := s.monitors.GetLastStatuses(m.ID)
	if err != nil {
		slog.Error("scheduler: get last statuses error", "monitor_id", m.ID, "error", err)
		return
	}

	// Respect per-monitor notification trigger settings.
	if result.Status == 0 && !m.NotifyOnFailure {
		return
	}
	if result.Status == 1 && !m.NotifyOnSuccess {
		return
	}

	// Skip if status did not change relative to last notification.
	if lastNotified != nil && *lastNotified == result.Status {
		return
	}

	notifs, err := s.notifications.ListForMonitor(m.ID)
	if err != nil || len(notifs) == 0 {
		// No notifications configured — still update the notified status so we
		// don't log errors on every check.
		if err != nil {
			slog.Error("scheduler: list notifications error", "monitor_id", m.ID, "error", err)
		}
		_ = s.monitors.UpdateLastNotifiedStatus(m.ID, result.Status)
		return
	}

	var configs []notifier.NotifConfig
	for _, n := range notifs {
		var cfg map[string]string
		if err := json.Unmarshal([]byte(n.Config), &cfg); err != nil {
			slog.Error("scheduler: bad notification config", "notification_id", n.ID, "error", err)
			continue
		}
		configs = append(configs, notifier.NotifConfig{
			ID:     n.ID,
			Name:   n.Name,
			Type:   n.Type,
			Config: cfg,
		})
	}

	msg := result.Message
	if result.BodyExcerpt != "" {
		msg = result.Message + "\n\nResponse body:\n" + result.BodyExcerpt
	}
	event := notifier.Event{
		MonitorID:   m.ID,
		MonitorName: m.Name,
		MonitorURL:  m.URL,
		Status:      result.Status,
		LatencyMs:   result.LatencyMs,
		Message:     msg,
	}

	results := notifier.SendAll(s.ctx, configs, event)

	now := time.Now().UTC()
	for _, r := range results {
		errStr := ""
		if r.Err != nil {
			errStr = r.Err.Error()
		}
		nid := r.NotifConfig.ID
		l := &models.NotificationLog{
			MonitorID:        &m.ID,
			NotificationID:   &nid,
			MonitorName:      m.Name,
			NotificationName: r.NotifConfig.Name,
			EventStatus:      result.Status,
			Success:          r.Err == nil,
			Error:            errStr,
			CreatedAt:        now,
		}
		if err := s.notifLogs.Insert(l); err != nil {
			slog.Error("scheduler: failed to insert notification log", "error", err)
		}
	}

	if err := s.monitors.UpdateLastNotifiedStatus(m.ID, result.Status); err != nil {
		slog.Error("scheduler: update last_notified_status error", "monitor_id", m.ID, "error", err)
	}
}

// RecordHeartbeat persists a push/heartbeat result for the given monitor and
// fires state-change notifications. Called by the unauthenticated /push/:token
// endpoint instead of the scheduler poller.
func (s *Scheduler) RecordHeartbeat(m *models.Monitor, status, latencyMs int, message string) {
	now := time.Now().UTC()

	// Get previous status for transition detection before recording new result.
	prevStatus, _, _ := s.monitors.GetLastStatuses(m.ID)

	h := &models.Heartbeat{
		MonitorID: m.ID,
		Status:    status,
		LatencyMs: latencyMs,
		Message:   message,
		CreatedAt: now,
	}
	if err := s.heartbeat.Insert(h); err != nil {
		slog.Error("scheduler: push heartbeat insert error", "monitor_id", m.ID, "error", err)
	}
	statusText := "UP"
	if status == 0 {
		statusText = "DOWN"
	}
	slog.Info("push monitor check", "monitor_id", m.ID, "monitor_name", m.Name, "status", statusText, "latency_ms", latencyMs, "message", message)

	// Track downtime events on state transitions.
	if status == 0 && (prevStatus == nil || *prevStatus != 0) {
		if err := s.downtimeEvents.OpenIncident(m.ID, now); err != nil {
			slog.Error("scheduler: open incident error", "monitor_id", m.ID, "error", err)
		}
	} else if status == 1 && prevStatus != nil && *prevStatus == 0 {
		if err := s.downtimeEvents.CloseIncident(m.ID, now); err != nil {
			slog.Error("scheduler: close incident error", "monitor_id", m.ID, "error", err)
		}
	}

	s.maybeNotify(m, monitor.Result{Status: status, LatencyMs: latencyMs, Message: message})
	if err := s.monitors.UpdateLastStatus(m.ID, status); err != nil {
		slog.Error("scheduler: push update last_status error", "monitor_id", m.ID, "error", err)
	}
}

// ---------------------------------------------------------------------------
// MultiScheduler — one Scheduler per user
// ---------------------------------------------------------------------------

// MultiScheduler manages a set of per-user Scheduler instances.
type MultiScheduler struct {
	mu         sync.RWMutex
	schedulers map[string]*Scheduler
}

// NewMulti creates a new MultiScheduler.
func NewMulti() *MultiScheduler {
	return &MultiScheduler{schedulers: make(map[string]*Scheduler)}
}

// StartForUser creates and starts a Scheduler for the given user using their DB.
// If a scheduler is already running for the user, this is a no-op.
func (ms *MultiScheduler) StartForUser(username string, db *sql.DB) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.schedulers[username]; exists {
		return
	}

	s := New(db)
	s.Start()
	ms.schedulers[username] = s
}

// ForUser returns the Scheduler for the given user, or nil if not yet started.
func (ms *MultiScheduler) ForUser(username string) *Scheduler {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.schedulers[username]
}

// StopUser stops and removes the scheduler for a single user.
func (ms *MultiScheduler) StopUser(username string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if s, ok := ms.schedulers[username]; ok {
		s.Stop()
		delete(ms.schedulers, username)
	}
}

// Stop stops all per-user schedulers.
func (ms *MultiScheduler) Stop() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, s := range ms.schedulers {
		s.Stop()
	}
}
