package models

import (
	"context"
	"database/sql"
	"time"
)

// RemoteBrowser stores a remote Chrome DevTools endpoint for browser monitors.
type RemoteBrowser struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	EndpointURL string    `db:"endpoint_url"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// RemoteBrowserStore handles CRUD for remote_browsers.
type RemoteBrowserStore struct {
	db *sql.DB
}

// NewRemoteBrowserStore creates a new RemoteBrowserStore.
func NewRemoteBrowserStore(db *sql.DB) *RemoteBrowserStore {
	return &RemoteBrowserStore{db: db}
}

// List returns all remote browser configs ordered by ID.
func (s *RemoteBrowserStore) List() ([]*RemoteBrowser, error) {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id, name, endpoint_url, created_at, updated_at
		FROM remote_browsers ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*RemoteBrowser
	for rows.Next() {
		rb := &RemoteBrowser{}
		if err := rows.Scan(&rb.ID, &rb.Name, &rb.EndpointURL, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, rb)
	}
	return items, rows.Err()
}

// Get returns one remote browser config by ID, or nil when not found.
func (s *RemoteBrowserStore) Get(id int64) (*RemoteBrowser, error) {
	rb := &RemoteBrowser{}
	err := s.db.QueryRowContext(context.Background(), `
		SELECT id, name, endpoint_url, created_at, updated_at
		FROM remote_browsers WHERE id = ?
	`, id).Scan(&rb.ID, &rb.Name, &rb.EndpointURL, &rb.CreatedAt, &rb.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rb, err
}

// Create inserts a new remote browser config and returns its ID.
func (s *RemoteBrowserStore) Create(rb *RemoteBrowser) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(context.Background(), `
		INSERT INTO remote_browsers (name, endpoint_url, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, rb.Name, rb.EndpointURL, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing remote browser config.
func (s *RemoteBrowserStore) Update(rb *RemoteBrowser) error {
	_, err := s.db.ExecContext(context.Background(), `
		UPDATE remote_browsers SET name = ?, endpoint_url = ?, updated_at = ? WHERE id = ?
	`, rb.Name, rb.EndpointURL, time.Now().UTC(), rb.ID)
	return err
}

// Delete removes a remote browser config by ID.
func (s *RemoteBrowserStore) Delete(id int64) error {
	_, err := s.db.ExecContext(context.Background(), `DELETE FROM remote_browsers WHERE id = ?`, id)
	return err
}
