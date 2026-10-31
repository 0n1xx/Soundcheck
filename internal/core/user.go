package core

import "time"

// User is someone who signed up for Soundcheck.
type User struct {
	ID           int64
	Email        string
	PasswordHash string // empty for people who sign in with Google
	City         string
	CreatedAt    time.Time
}
