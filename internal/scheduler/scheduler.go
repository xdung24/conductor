package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
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
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start loads all active monitors and begins scheduling them.
func (s *Scheduler) Start() {
	monitors, err := s.monitors.List()
	if err != nil {
		log.Printf("scheduler: failed to load monitors: %v", err)
		return
	}

	for _, m := range monitors {
		if m.Active {
			s.Schedule(m)
		}
	}
	log.Printf("scheduler: started %d monitor(s)", len(s.jobs))
}

// Stop cancels all running jobs.
func (s *Scheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		close(j.stop)
	}
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
					log.Printf("monitor[%d] %s — skipped (maintenance window active)", latest.ID, latest.Name)
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

// Unschedule stops the job for a given monitor ID.
func (s *Scheduler) Unschedule(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		close(j.stop)
		delete(s.jobs, id)
	}
}

func (s *Scheduler) runCheck(m *models.Monitor) {
	result := monitor.Run(s.ctx, s.db, m)

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
		log.Printf("scheduler: failed to save heartbeat for monitor %d: %v", m.ID, err)
	}

	statusText := "UP"
	if result.Status == 0 {
		statusText = "DOWN"
	}
	log.Printf("monitor[%d] %s — %s (%dms) %s", m.ID, m.Name, statusText, result.LatencyMs, result.Message)

	// Track downtime events on state transitions.
	if result.Status == 0 && (prevStatus == nil || *prevStatus != 0) {
		if err := s.downtimeEvents.OpenIncident(m.ID, now); err != nil {
			log.Printf("scheduler: open incident for monitor %d: %v", m.ID, err)
		}
	} else if result.Status == 1 && prevStatus != nil && *prevStatus == 0 {
		if err := s.downtimeEvents.CloseIncident(m.ID, now); err != nil {
			log.Printf("scheduler: close incident for monitor %d: %v", m.ID, err)
		}
	}

	// State-change detection — only notify when status flips.
	s.maybeNotify(m, result)

	// Persist the last status for the next comparison.
	if err := s.monitors.UpdateLastStatus(m.ID, result.Status); err != nil {
		log.Printf("scheduler: failed to update last_status for monitor %d: %v", m.ID, err)
	}
}

// maybeNotify fires notifications only when the monitor changes state.
func (s *Scheduler) maybeNotify(m *models.Monitor, result monitor.Result) {
	_, lastNotified, err := s.monitors.GetLastStatuses(m.ID)
	if err != nil {
		log.Printf("scheduler: get last statuses for monitor %d: %v", m.ID, err)
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
			log.Printf("scheduler: list notifications for monitor %d: %v", m.ID, err)
		}
		_ = s.monitors.UpdateLastNotifiedStatus(m.ID, result.Status)
		return
	}

	var configs []notifier.NotifConfig
	for _, n := range notifs {
		var cfg map[string]string
		if err := json.Unmarshal([]byte(n.Config), &cfg); err != nil {
			log.Printf("scheduler: bad config for notification %d: %v", n.ID, err)
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
			log.Printf("scheduler: failed to insert notification log: %v", err)
		}
	}

	if err := s.monitors.UpdateLastNotifiedStatus(m.ID, result.Status); err != nil {
		log.Printf("scheduler: update last_notified_status for monitor %d: %v", m.ID, err)
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
		log.Printf("scheduler: push heartbeat insert for monitor %d: %v", m.ID, err)
	}
	statusText := "UP"
	if status == 0 {
		statusText = "DOWN"
	}
	log.Printf("push[%d] %s — %s (%dms) %s", m.ID, m.Name, statusText, latencyMs, message)

	// Track downtime events on state transitions.
	if status == 0 && (prevStatus == nil || *prevStatus != 0) {
		if err := s.downtimeEvents.OpenIncident(m.ID, now); err != nil {
			log.Printf("scheduler: open incident for monitor %d: %v", m.ID, err)
		}
	} else if status == 1 && prevStatus != nil && *prevStatus == 0 {
		if err := s.downtimeEvents.CloseIncident(m.ID, now); err != nil {
			log.Printf("scheduler: close incident for monitor %d: %v", m.ID, err)
		}
	}

	s.maybeNotify(m, monitor.Result{Status: status, LatencyMs: latencyMs, Message: message})
	if err := s.monitors.UpdateLastStatus(m.ID, status); err != nil {
		log.Printf("scheduler: push update last_status for monitor %d: %v", m.ID, err)
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
