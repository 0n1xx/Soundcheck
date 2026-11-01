package store

import (
	"context"
	"fmt"
	"time"

	"github.com/0n1xx/soundcheck/internal/core"
)

// maxShows keeps the Upcoming shows page a reasonable size.
const maxShows = 200

// UpcomingShows returns concerts of the artists a user follows in the given
// city, soonest first. An empty city means "every city".
// Shows that started more than 12 hours ago are left out, so tonight's
// concert stays on the list until tomorrow morning.
func (s *Store) UpcomingShows(ctx context.Context, userID int64, city string) ([]core.Event, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, artist_id, name, venue, city, url,
		        starts_at, on_sale_at, status, timezone
		 FROM events
		 WHERE artist_id IN (SELECT tm_id FROM user_artists WHERE user_id = $1)
		   AND ($2 = '' OR lower(city) = lower($2))
		   AND (starts_at IS NULL OR starts_at > now() - interval '12 hours')
		 ORDER BY starts_at NULLS LAST
		 LIMIT $3`,
		userID, city, maxShows,
	)
	if err != nil {
		return nil, fmt.Errorf("upcoming shows: %w", err)
	}
	defer rows.Close()

	var shows []core.Event
	for rows.Next() {
		var e core.Event
		// Dates may be NULL in the database, so read them into pointers first.
		var startsAt, onSaleAt *time.Time
		err := rows.Scan(&e.ID, &e.ArtistID, &e.Name, &e.Venue, &e.City, &e.URL,
			&startsAt, &onSaleAt, &e.Status, &e.Timezone)
		if err != nil {
			return nil, fmt.Errorf("read show: %w", err)
		}
		if startsAt != nil {
			e.StartsAt = *startsAt
		}
		if onSaleAt != nil {
			e.OnSaleAt = *onSaleAt
		}
		shows = append(shows, e)
	}
	return shows, rows.Err()
}
