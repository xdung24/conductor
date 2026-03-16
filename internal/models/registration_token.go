package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// RegistrationToken represents a single-use invite link created by an admin.
type RegistrationToken struct {
	Token     string
	CreatedBy string
	UsedAt    *time.Time
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// Pending returns true when the token has not yet been consumed and has not expired.
func (t *RegistrationToken) Pending() bool {
	if t.UsedAt != nil {
		return false
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return false
	}
	return true
}

// RegistrationTokenStore manages invite tokens in the shared users DB.
type RegistrationTokenStore struct {
	db *sql.DB
}

// NewRegistrationTokenStore creates a RegistrationTokenStore.
func NewRegistrationTokenStore(db *sql.DB) *RegistrationTokenStore {
	return &RegistrationTokenStore{db: db}
}

// Generate creates a new random token owned by createdBy and persists it.
// Pass ttl > 0 to set an expiry (e.g. 30*time.Minute for the startup token).
// Pass ttl == 0 for a non-expiring admin invite link.
// The returned string is the raw token value to embed in the invite URL.
func (s *RegistrationTokenStore) Generate(createdBy string, ttl time.Duration) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().UTC().Add(ttl)
		expiresAt = &t
	}

	_, err := s.db.Exec(
		`INSERT INTO registration_tokens (token, created_by, expires_at) VALUES (?, ?, ?)`,
		token, createdBy, expiresAt,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetPending returns a token record only when it exists, has not been used,
// and has not expired.
func (s *RegistrationTokenStore) GetPending(token string) (*RegistrationToken, error) {
	rt := &RegistrationToken{}
	var usedAt sql.NullTime
	var expiresAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT token, created_by, used_at, expires_at, created_at
		 FROM registration_tokens WHERE token = ?`,
		token,
	).Scan(&rt.Token, &rt.CreatedBy, &usedAt, &expiresAt, &rt.CreatedAt)
	if err != nil {
		return nil, err
	}
	if usedAt.Valid {
		return nil, sql.ErrNoRows // already used
	}
	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		return nil, sql.ErrNoRows // expired
	}
	if expiresAt.Valid {
		rt.ExpiresAt = &expiresAt.Time
	}
	return rt, nil
}

// Consume marks the token as used. Must be called inside the same transaction
// that creates the new user account to keep the two operations atomic; here we
// use a simple sequential approach (tokens are single-use by design).
func (s *RegistrationTokenStore) Consume(token string) error {
	res, err := s.db.Exec(
		`UPDATE registration_tokens SET used_at = ? WHERE token = ? AND used_at IS NULL`,
		time.Now().UTC(), token,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows // already consumed or missing
	}
	return nil
}

// ListAll returns every token ordered newest-first.
func (s *RegistrationTokenStore) ListAll() ([]*RegistrationToken, error) {
	rows, err := s.db.Query(
		`SELECT token, created_by, used_at, expires_at, created_at
		 FROM registration_tokens ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*RegistrationToken
	for rows.Next() {
		rt := &RegistrationToken{}
		var usedAt sql.NullTime
		var expiresAt sql.NullTime
		if err := rows.Scan(&rt.Token, &rt.CreatedBy, &usedAt, &expiresAt, &rt.CreatedAt); err != nil {
			return nil, err
		}
		if usedAt.Valid {
			rt.UsedAt = &usedAt.Time
		}
		if expiresAt.Valid {
			rt.ExpiresAt = &expiresAt.Time
		}
		tokens = append(tokens, rt)
	}
	return tokens, rows.Err()
}

// Delete removes a token. Any admin can revoke any token.
func (s *RegistrationTokenStore) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM registration_tokens WHERE token = ?`, token)
	return err
}
