package models

import (
	"context"
	"database/sql"
	"time"
)

// Proxy stores connection details for an HTTP/HTTPS proxy server.
// A monitor with type "http" may reference a Proxy via ProxyID.
type Proxy struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"` // human-readable label
	URL       string    `db:"url"`  // full proxy URL, e.g. http://user:pass@host:8080
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ProxyStore handles CRUD for proxies.
type ProxyStore struct {
	db *sql.DB
}

// NewProxyStore creates a new ProxyStore.
func NewProxyStore(db *sql.DB) *ProxyStore {
	return &ProxyStore{db: db}
}

// List returns all proxies ordered by ID.
func (s *ProxyStore) List() ([]*Proxy, error) {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id, name, url, created_at, updated_at
		FROM proxies ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []*Proxy
	for rows.Next() {
		p := &Proxy{}
		if err := rows.Scan(&p.ID, &p.Name, &p.URL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		proxies = append(proxies, p)
	}
	return proxies, rows.Err()
}

// Get returns a single Proxy by ID, or nil if not found.
func (s *ProxyStore) Get(id int64) (*Proxy, error) {
	p := &Proxy{}
	err := s.db.QueryRowContext(context.Background(), `
		SELECT id, name, url, created_at, updated_at
		FROM proxies WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.URL, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// Create inserts a new Proxy and returns its ID.
func (s *ProxyStore) Create(p *Proxy) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(context.Background(), `
		INSERT INTO proxies (name, url, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, p.Name, p.URL, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing Proxy.
func (s *ProxyStore) Update(p *Proxy) error {
	_, err := s.db.ExecContext(context.Background(), `
		UPDATE proxies SET name=?, url=?, updated_at=? WHERE id=?
	`, p.Name, p.URL, time.Now().UTC(), p.ID)
	return err
}

// Delete removes a Proxy by ID.
func (s *ProxyStore) Delete(id int64) error {
	_, err := s.db.ExecContext(context.Background(), `DELETE FROM proxies WHERE id = ?`, id)
	return err
}
