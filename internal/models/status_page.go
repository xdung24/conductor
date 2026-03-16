package models

import (
	"context"
	"database/sql"
	"time"
)

// StatusPage is a public read-only page that shows the status of selected monitors.
type StatusPage struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`        // URL-friendly identifier, globally unique per user
	Description string    `db:"description"` // optional subtitle shown on the public page
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// StatusPageMonitorEntry holds display data for a monitor on a public status page.
type StatusPageMonitorEntry struct {
	Monitor   *Monitor
	Uptime24h float64
}

// StatusPageStore handles all status-page DB operations.
type StatusPageStore struct {
	db *sql.DB
}

// NewStatusPageStore creates a new StatusPageStore.
func NewStatusPageStore(db *sql.DB) *StatusPageStore {
	return &StatusPageStore{db: db}
}

// List returns all status pages ordered by name.
func (s *StatusPageStore) List() ([]*StatusPage, error) {
	rows, err := s.db.Query(`
		SELECT id, name, slug, description, created_at, updated_at
		FROM status_pages ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*StatusPage
	for rows.Next() {
		p := &StatusPage{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// Get returns a single status page by ID.
func (s *StatusPageStore) Get(id int64) (*StatusPage, error) {
	p := &StatusPage{}
	err := s.db.QueryRow(`
		SELECT id, name, slug, description, created_at, updated_at
		FROM status_pages WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// GetBySlug returns a status page by its URL slug.
func (s *StatusPageStore) GetBySlug(slug string) (*StatusPage, error) {
	p := &StatusPage{}
	err := s.db.QueryRow(`
		SELECT id, name, slug, description, created_at, updated_at
		FROM status_pages WHERE slug = ?
	`, slug).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// Create inserts a new status page and returns its ID.
func (s *StatusPageStore) Create(p *StatusPage) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(context.Background(), `
		INSERT INTO status_pages (name, slug, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, p.Name, p.Slug, p.Description, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing status page.
func (s *StatusPageStore) Update(p *StatusPage) error {
	_, err := s.db.ExecContext(context.Background(), `
		UPDATE status_pages SET name=?, slug=?, description=?, updated_at=? WHERE id=?
	`, p.Name, p.Slug, p.Description, time.Now().UTC(), p.ID)
	return err
}

// Delete removes a status page and all its monitor associations.
func (s *StatusPageStore) Delete(id int64) error {
	_, err := s.db.ExecContext(context.Background(), `DELETE FROM status_pages WHERE id = ?`, id)
	return err
}

// SetMonitors replaces the monitor list for a status page.
func (s *StatusPageStore) SetMonitors(pageID int64, monitorIDs []int64) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM status_page_monitors WHERE page_id = ?`, pageID); err != nil {
		return err
	}
	for pos, mid := range monitorIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO status_page_monitors (page_id, monitor_id, position) VALUES (?, ?, ?)`,
			pageID, mid, pos,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListMonitorIDs returns monitor IDs linked to a status page, ordered by position.
func (s *StatusPageStore) ListMonitorIDs(pageID int64) ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT monitor_id FROM status_page_monitors
		WHERE page_id = ? ORDER BY position ASC
	`, pageID)
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
