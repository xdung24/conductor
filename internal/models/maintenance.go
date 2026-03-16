package models

import (
	"context"
	"database/sql"
	"time"
)

// MaintenanceWindow defines a time range during which monitor alerts are suppressed.
type MaintenanceWindow struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	StartTime time.Time `db:"start_time"`
	EndTime   time.Time `db:"end_time"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// Computed field: monitor IDs linked to this window (not in DB row).
	MonitorIDs []int64 `db:"-"`
}

// MaintenanceStore handles all maintenance-window DB operations.
type MaintenanceStore struct {
	db *sql.DB
}

// NewMaintenanceStore creates a new MaintenanceStore.
func NewMaintenanceStore(db *sql.DB) *MaintenanceStore {
	return &MaintenanceStore{db: db}
}

func (s *MaintenanceStore) scan(rows *sql.Rows) (*MaintenanceWindow, error) {
	w := &MaintenanceWindow{}
	if err := rows.Scan(&w.ID, &w.Name, &w.StartTime, &w.EndTime, &w.Active, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return nil, err
	}
	return w, nil
}

// List returns all maintenance windows ordered by start_time.
func (s *MaintenanceStore) List() ([]*MaintenanceWindow, error) {
	rows, err := s.db.Query(`
		SELECT id, name, start_time, end_time, active, created_at, updated_at
		FROM maintenance_windows ORDER BY start_time ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []*MaintenanceWindow
	for rows.Next() {
		w, err := s.scan(rows)
		if err != nil {
			return nil, err
		}
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

// Get returns a single maintenance window by ID.
func (s *MaintenanceStore) Get(id int64) (*MaintenanceWindow, error) {
	rows, err := s.db.Query(`
		SELECT id, name, start_time, end_time, active, created_at, updated_at
		FROM maintenance_windows WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}
	return s.scan(rows)
}

// Create inserts a new maintenance window and returns its ID.
func (s *MaintenanceStore) Create(w *MaintenanceWindow) (int64, error) {
	n := time.Now().UTC()
	res, err := s.db.ExecContext(context.Background(), `
		INSERT INTO maintenance_windows (name, start_time, end_time, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, w.Name, w.StartTime, w.EndTime, w.Active, n, n)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing maintenance window.
func (s *MaintenanceStore) Update(w *MaintenanceWindow) error {
	_, err := s.db.ExecContext(context.Background(), `
		UPDATE maintenance_windows SET name=?, start_time=?, end_time=?, active=?, updated_at=?
		WHERE id=?
	`, w.Name, w.StartTime, w.EndTime, w.Active, time.Now().UTC(), w.ID)
	return err
}

// Delete removes a maintenance window and all its monitor associations.
func (s *MaintenanceStore) Delete(id int64) error {
	_, err := s.db.ExecContext(context.Background(), `DELETE FROM maintenance_windows WHERE id = ?`, id)
	return err
}

// SetMonitors replaces the monitor list for a maintenance window.
func (s *MaintenanceStore) SetMonitors(windowID int64, monitorIDs []int64) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM monitor_maintenance WHERE window_id = ?`, windowID); err != nil {
		return err
	}
	for _, mid := range monitorIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO monitor_maintenance (window_id, monitor_id) VALUES (?, ?)`,
			windowID, mid,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListMonitorIDs returns all monitor IDs linked to a window.
func (s *MaintenanceStore) ListMonitorIDs(windowID int64) ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT monitor_id FROM monitor_maintenance WHERE window_id = ?
	`, windowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsInMaintenance reports whether monitor `monitorID` is covered by any active
// maintenance window at time `t`.
func (s *MaintenanceStore) IsInMaintenance(monitorID int64, t time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM maintenance_windows mw
		JOIN monitor_maintenance mm ON mm.window_id = mw.id
		WHERE mm.monitor_id = ?
		  AND mw.active = 1
		  AND mw.start_time <= ?
		  AND mw.end_time   >= ?
	`, monitorID, t, t).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
