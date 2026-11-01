package store

import (
	"context"
	"fmt"
)

// Cleanup deletes data nobody needs any more, so the tables don't grow
// forever. The worker calls it once per round.
//
// Each statement is safe to run again and again: it only removes rows
// that are already expired or long in the past.
func (s *Store) Cleanup(ctx context.Context) error {
	statements := []struct {
		what string
		sql  string
	}{
		// Logins that ran out (30 days old) or were never used again.
		{"expired sessions", `DELETE FROM sessions WHERE expires_at < now()`},

		// "Connect Telegram" links live for 10 minutes.
		{"expired telegram links", `DELETE FROM telegram_links WHERE expires_at < now()`},

		// Sign-ups nobody confirmed within a week. They could never log in,
		// so they have no artists; the checks keep anyone else safe.
		{"unconfirmed sign-ups", `DELETE FROM users u
			WHERE NOT u.email_verified AND u.google_id IS NULL
			  AND u.created_at < now() - interval '7 days'
			  AND NOT EXISTS (SELECT 1 FROM user_artists WHERE user_id = u.id)
			  AND NOT EXISTS (SELECT 1 FROM sessions WHERE user_id = u.id)`},

		// Email confirmation links live for 24 hours.
		{"expired email links", `DELETE FROM email_tokens WHERE expires_at < now()`},
		{"expired reset links", `DELETE FROM password_resets WHERE expires_at < now()`},

		// Concerts that ended over a month ago, and the alerts sent about them.
		// The alerts go first: they point at the concerts by ID.
		{"old alerts", `DELETE FROM sent_alerts WHERE event_id IN (
			SELECT id FROM events WHERE starts_at < now() - interval '30 days')`},
		{"old concerts", `DELETE FROM events WHERE starts_at < now() - interval '30 days'`},
	}

	for _, st := range statements {
		if _, err := s.pool.Exec(ctx, st.sql); err != nil {
			return fmt.Errorf("clean up %s: %w", st.what, err)
		}
	}
	return nil
}
