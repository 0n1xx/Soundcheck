package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0n1xx/soundcheck/internal/core"
)

// EventChange tells the worker what happened when it saved a concert,
// so it knows which email (if any) to send.
type EventChange int

const (
	EventUnchanged     EventChange = iota // already known, nothing important changed
	EventNew                              // seen for the first time
	EventStatusChanged                    // e.g. onsale -> cancelled
	EventRescheduled                      // the date moved
)

// SaveEvent stores a concert found by the worker and reports what changed.
// The same concert is found on every run, so this inserts it the first time
// and updates it afterwards, never creating duplicates.
func (s *Store) SaveEvent(ctx context.Context, e core.Event) (EventChange, error) {
	// 1. Try to insert. ON CONFLICT DO NOTHING means: if a concert with this
	//    ID already exists, skip quietly instead of failing.
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO events (id, artist_id, name, venue, city, url, starts_at, on_sale_at, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (id) DO NOTHING`,
		e.ID, e.ArtistID, e.Name, e.Venue, e.City, e.URL,
		nullTime(e.StartsAt), nullTime(e.OnSaleAt), e.Status,
	)
	if err != nil {
		return EventUnchanged, fmt.Errorf("insert event %s: %w", e.ID, err)
	}
	// One row inserted means we've never seen this concert before.
	if tag.RowsAffected() == 1 {
		return EventNew, nil
	}

	// 2. The concert is already known: read what we had before...
	var oldStatus string
	var oldStartsAt *time.Time
	err = s.pool.QueryRow(ctx,
		`SELECT status, starts_at FROM events WHERE id = $1`,
		e.ID,
	).Scan(&oldStatus, &oldStartsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return EventUnchanged, nil // deleted in between; nothing to compare
	}
	if err != nil {
		return EventUnchanged, fmt.Errorf("read event %s: %w", e.ID, err)
	}

	// 3. ...and save the fresh details from Ticketmaster.
	_, err = s.pool.Exec(ctx,
		`UPDATE events
		 SET name = $2, venue = $3, city = $4, url = $5,
		     starts_at = $6, on_sale_at = $7, status = $8, updated_at = now()
		 WHERE id = $1`,
		e.ID, e.Name, e.Venue, e.City, e.URL,
		nullTime(e.StartsAt), nullTime(e.OnSaleAt), e.Status,
	)
	if err != nil {
		return EventUnchanged, fmt.Errorf("update event %s: %w", e.ID, err)
	}

	// 4. Compare old and new to tell the worker what happened.
	switch {
	case oldStatus != e.Status:
		return EventStatusChanged, nil
	case !sameTime(oldStartsAt, e.StartsAt):
		return EventRescheduled, nil
	default:
		return EventUnchanged, nil
	}
}

// nullTime turns an unknown (zero) time into NULL for the database.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// sameTime compares a time from the database (maybe NULL) with a new one.
func sameTime(old *time.Time, t time.Time) bool {
	if old == nil {
		return t.IsZero()
	}
	return old.Equal(t)
}
