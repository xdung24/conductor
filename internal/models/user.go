package models

import "time"

// User represents a dashboard user.
type User struct {
	ID        int64     `db:"id"`
	Username  string    `db:"username"`
	Password  string    `db:"password"`
	CreatedAt time.Time `db:"created_at"`
	IsAdmin   bool      `db:"is_admin"`
}
