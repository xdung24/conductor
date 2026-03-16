package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// APIKey represents an API key record.
// The plain token is shown exactly once on creation; only its SHA-256 hash is persisted.
type APIKey struct {
	ID         int64      `db:"id"`
	Username   string     `db:"username"`
	Name       string     `db:"name"`
	TokenHash  string     `db:"token_hash"`
	CreatedAt  time.Time  `db:"created_at"`
	LastUsedAt *time.Time `db:"last_used_at"`
}

// APIKeyStore manages API keys backed by the shared users DB.
type APIKeyStore struct {
	db *sql.DB
}

// NewAPIKeyStore creates a new APIKeyStore.
func NewAPIKeyStore(db *sql.DB) *APIKeyStore {
	return &APIKeyStore{db: db}
}

// GenerateAPIToken returns a cryptographically random 32-byte hex token (64 chars).
func GenerateAPIToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashAPIToken returns the hex-encoded SHA-256 of the plain-text token.
// Using SHA-256 (not bcrypt) is sound here: the token itself is already
// 256 bits of entropy, so hash-speed attacks are infeasible.
func HashAPIToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// List returns all API keys for the given user.
func (s *APIKeyStore) List(username string) ([]*APIKey, error) {
	rows, err := s.db.Query(`
		SELECT id, username, name, token_hash, created_at, last_used_at
		FROM api_keys WHERE username=? ORDER BY id ASC
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		k := &APIKey{}
		if err := rows.Scan(&k.ID, &k.Username, &k.Name, &k.TokenHash, &k.CreatedAt, &k.LastUsedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// Create inserts a new API key with the pre-hashed token and returns its ID.
func (s *APIKeyStore) Create(username, name, tokenHash string) (int64, error) {
	res, err := s.db.ExecContext(context.Background(), `
		INSERT INTO api_keys (username, name, token_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, username, name, tokenHash, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Delete removes an API key by ID, verifying ownership by username.
func (s *APIKeyStore) Delete(id int64, username string) error {
	_, err := s.db.ExecContext(context.Background(),
		`DELETE FROM api_keys WHERE id=? AND username=?`, id, username)
	return err
}

// Verify looks up a plain-text token by its hash, updates last_used_at,
// and returns the owning username. Returns an error if not found.
func (s *APIKeyStore) Verify(plainToken string) (string, error) {
	hash := HashAPIToken(plainToken)
	var id int64
	var username string
	err := s.db.QueryRow(`SELECT id, username FROM api_keys WHERE token_hash=?`, hash).
		Scan(&id, &username)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid api key")
	}
	if err != nil {
		return "", err
	}
	_, _ = s.db.ExecContext(context.Background(),
		`UPDATE api_keys SET last_used_at=? WHERE id=?`, time.Now(), id)
	return username, nil
}
