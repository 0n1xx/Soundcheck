package store

import (
	"context"
	"fmt"
	"time"

	"github.com/0n1xx/soundcheck/internal/core"
)

// NewShowFollowers returns the users to tell about a newly found concert:
// they follow its artist, their city is the concert's city, and they
// followed the artist before the concert appeared. The last rule skips the
// shows that were already announced when someone added the artist; those
// are on the Upcoming shows page anyway.
func (s *Store) NewShowFollowers(ctx context.Context, eventID string) ([]core.User, error) {
	return s.followers(ctx, eventID, `AND ua.created_at < e.first_seen - interval '1 minute'`)
}

// EventFollowers returns everyone who follows the concert's artist and lives
// in its city. Used for cancellations and new dates: everyone should know.
func (s *Store) EventFollowers(ctx context.Context, eventID string) ([]core.User, error) {
	return s.followers(ctx, eventID, "")
}

// followers finds the users who follow an event's artist in the event's city.
// extra is an optional SQL condition written in this file, never user input.
func (s *Store) followers(ctx context.Context, eventID, extra string) ([]core.User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT u.id, u.email, COALESCE(u.telegram_chat_id, 0)
		 FROM events e
		 JOIN user_artists ua ON ua.tm_id = e.artist_id
		 JOIN users u ON u.id = ua.user_id
		 WHERE e.id = $1
		   AND u.city <> '' AND lower(u.city) = lower(e.city) `+extra,
		eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("followers of %s: %w", eventID, err)
	}
	defer rows.Close()

	var users []core.User
	for rows.Next() {
		var u core.User
		if err := rows.Scan(&u.ID, &u.Email, &u.TelegramChatID); err != nil {
			return nil, fmt.Errorf("read follower: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// MarkAlertSent records that a user is getting an alert about an event.
// It returns false if that alert was already sent, so it isn't sent twice.
// Call it before sending: it's like taking a ticket at the counter.
func (s *Store) MarkAlertSent(ctx context.Context, userID int64, eventID, kind string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO sent_alerts (user_id, event_id, kind) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		userID, eventID, kind,
	)
	if err != nil {
		return false, fmt.Errorf("mark alert sent: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// UnmarkAlert forgets an alert that failed to send, so it can be retried.
func (s *Store) UnmarkAlert(ctx context.Context, userID int64, eventID, kind string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM sent_alerts WHERE user_id = $1 AND event_id = $2 AND kind = $3`,
		userID, eventID, kind,
	)
	if err != nil {
		return fmt.Errorf("unmark alert: %w", err)
	}
	return nil
}

// Reminder is one alert waiting to be sent: which concert, to whom.
type Reminder struct {
	Event core.Event
	User  core.User
}

// OnSaleReminders finds concerts whose public sale starts between from and
// to, paired with every follower in the concert's city who hasn't had this
// kind of reminder yet.
func (s *Store) OnSaleReminders(ctx context.Context, from, to time.Time, kind string) ([]Reminder, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT e.id, e.name, e.venue, e.city, e.url,
		        e.starts_at, e.on_sale_at, e.status, e.timezone,
		        u.id, u.email, COALESCE(u.telegram_chat_id, 0)
		 FROM events e
		 JOIN user_artists ua ON ua.tm_id = e.artist_id
		 JOIN users u ON u.id = ua.user_id
		 WHERE e.on_sale_at > $1 AND e.on_sale_at <= $2
		   AND e.status <> 'cancelled'
		   AND u.city <> '' AND lower(u.city) = lower(e.city)
		   AND NOT EXISTS (
		       SELECT 1 FROM sent_alerts sa
		       WHERE sa.user_id = u.id AND sa.event_id = e.id AND sa.kind = $3
		   )`,
		from, to, kind,
	)
	if err != nil {
		return nil, fmt.Errorf("on-sale reminders: %w", err)
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var r Reminder
		var startsAt, onSaleAt *time.Time // may be NULL
		err := rows.Scan(&r.Event.ID, &r.Event.Name, &r.Event.Venue, &r.Event.City, &r.Event.URL,
			&startsAt, &onSaleAt, &r.Event.Status, &r.Event.Timezone,
			&r.User.ID, &r.User.Email, &r.User.TelegramChatID)
		if err != nil {
			return nil, fmt.Errorf("read reminder: %w", err)
		}
		if startsAt != nil {
			r.Event.StartsAt = *startsAt
		}
		if onSaleAt != nil {
			r.Event.OnSaleAt = *onSaleAt
		}
		reminders = append(reminders, r)
	}
	return reminders, rows.Err()
}
