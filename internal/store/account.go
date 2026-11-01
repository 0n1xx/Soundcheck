package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Notifications is what the Notifications window in Settings saves.
type Notifications struct {
	New     bool // a new show is announced
	Day     bool // a day before tickets go on sale
	Minutes bool // 15 minutes before tickets go on sale
	Changes bool // a show is cancelled, postponed or moved
}

// SetEmail changes a user's email, or returns ErrEmailTaken. A new address
// isn't verified yet; saving the same address again keeps it verified.
// (On the right of SET, "email" is still the old value.)
func (s *Store) SetEmail(ctx context.Context, userID int64, email string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users
		 SET email = $1, email_verified = (email = $1 AND email_verified)
		 WHERE id = $2`,
		normalizeEmail(email), userID,
	)
	// 23505: another account already has this email (the column is UNIQUE).
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("set email: %w", err)
	}
	return nil
}

// SetPassword saves a new password hash.
func (s *Store) SetPassword(ctx context.Context, userID int64, hash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		hash, userID,
	)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	return nil
}

// SetNotifications saves which alerts a user wants.
func (s *Store) SetNotifications(ctx context.Context, userID int64, n Notifications) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users
		 SET notify_new = $1, notify_24h = $2, notify_15m = $3, notify_changes = $4
		 WHERE id = $5`,
		n.New, n.Day, n.Minutes, n.Changes, userID,
	)
	if err != nil {
		return fmt.Errorf("set notifications: %w", err)
	}
	return nil
}

// DeleteUser removes a user for good. ON DELETE CASCADE in the schema
// removes their sessions, artists, Telegram links and sent alerts too.
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
