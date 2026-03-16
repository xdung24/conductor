package models

import (
	"context"
	"database/sql"
	"time"
)

// DockerHost stores connection details for a Docker daemon.
// A monitor with type "docker" references a DockerHost via DockerHostID.
type DockerHost struct {
	ID         int64     `db:"id"`
	Name       string    `db:"name"`        // human-readable label
	SocketPath string    `db:"socket_path"` // Unix socket path, e.g. /var/run/docker.sock; empty = use http_url
	HTTPURL    string    `db:"http_url"`    // TCP API URL, e.g. http://host:2375 or https://host:2376
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// DockerHostStore handles CRUD for docker_hosts.
type DockerHostStore struct {
	db *sql.DB
}

// NewDockerHostStore creates a new DockerHostStore.
func NewDockerHostStore(db *sql.DB) *DockerHostStore {
	return &DockerHostStore{db: db}
}

// List returns all Docker hosts ordered by ID.
func (s *DockerHostStore) List() ([]*DockerHost, error) {
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT id, name, socket_path, http_url, created_at, updated_at
		FROM docker_hosts ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []*DockerHost
	for rows.Next() {
		h := &DockerHost{}
		if err := rows.Scan(&h.ID, &h.Name, &h.SocketPath, &h.HTTPURL, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

// Get returns a single DockerHost by ID, or nil if not found.
func (s *DockerHostStore) Get(id int64) (*DockerHost, error) {
	h := &DockerHost{}
	err := s.db.QueryRowContext(context.Background(), `
		SELECT id, name, socket_path, http_url, created_at, updated_at
		FROM docker_hosts WHERE id = ?
	`, id).Scan(&h.ID, &h.Name, &h.SocketPath, &h.HTTPURL, &h.CreatedAt, &h.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

// Create inserts a new DockerHost and returns its ID.
func (s *DockerHostStore) Create(h *DockerHost) (int64, error) {
	now := time.Now()
	res, err := s.db.ExecContext(context.Background(), `
		INSERT INTO docker_hosts (name, socket_path, http_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, h.Name, h.SocketPath, h.HTTPURL, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update modifies an existing DockerHost.
func (s *DockerHostStore) Update(h *DockerHost) error {
	_, err := s.db.ExecContext(context.Background(), `
		UPDATE docker_hosts SET name=?, socket_path=?, http_url=?, updated_at=? WHERE id=?
	`, h.Name, h.SocketPath, h.HTTPURL, time.Now(), h.ID)
	return err
}

// Delete removes a DockerHost by ID.
func (s *DockerHostStore) Delete(id int64) error {
	_, err := s.db.ExecContext(context.Background(), `DELETE FROM docker_hosts WHERE id=?`, id)
	return err
}
