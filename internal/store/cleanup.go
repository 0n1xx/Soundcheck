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
