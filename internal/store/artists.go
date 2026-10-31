package store

import (
	"context"
	"fmt"

	"github.com/0n1xx/soundcheck/internal/core"
)

// ListArtists returns the artists a user follows, oldest first.
// It's used to fill the Artists page and the counters in the sidebar.
func (s *Store) ListArtists(ctx context.Context, userID int64) ([]core.Artist, error) {
	// Query is for SELECTs that can return many rows.
	rows, err := s.pool.Query(ctx,
		`SELECT name, tm_id FROM user_artists
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	// The rows hold a database connection; give it back to the pool when done.
	defer rows.Close()

	var artists []core.Artist

	// Read the result one row at a time, like while (reader.Read()) in C#.
	for rows.Next() {
		var a core.Artist
		if err := rows.Scan(&a.Name, &a.TicketmasterID); err != nil {
			return nil, fmt.Errorf("scan artist: %w", err)
		}
		artists = append(artists, a)
	}

	// The loop also stops if the connection breaks halfway,
	// so check whether it ended because of an error.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read artists: %w", err)
	}
	return artists, nil
}

// AddArtist makes a user follow an artist.
// Adding the same artist twice does nothing instead of failing.
func (s *Store) AddArtist(ctx context.Context, userID int64, a core.Artist) error {
	// ON CONFLICT relies on PRIMARY KEY (user_id, name) from schema.go:
	// if this pair already exists, Postgres skips the insert quietly.
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_artists (user_id, name, tm_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, name) DO NOTHING`,
		userID, a.Name, a.TicketmasterID,
	)
	if err != nil {
		return fmt.Errorf("add artist: %w", err)
	}
	return nil
}

// RemoveArtist stops a user following an artist.
func (s *Store) RemoveArtist(ctx context.Context, userID int64, name string) error {
	// user_id is always part of the WHERE, so people can only ever
	// remove their own artists, even if someone tampers with the form.
	_, err := s.pool.Exec(ctx,
		`DELETE FROM user_artists WHERE user_id = $1 AND name = $2`,
		userID, name,
	)
	if err != nil {
		return fmt.Errorf("remove artist: %w", err)
	}
	return nil
}

// SetCity changes the city a user wants shows in.
// The worker will later use it to filter concerts by location.
func (s *Store) SetCity(ctx context.Context, userID int64, city string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET city = $1 WHERE id = $2`,
		city, userID,
	)
	if err != nil {
		return fmt.Errorf("set city: %w", err)
	}
	return nil
}

// ArtistIDsToCheck returns every Ticketmaster artist ID that at least one
// user follows. Each artist appears once, even if many users follow them,
// so the worker asks Ticketmaster about each artist only once per run.
func (s *Store) ArtistIDsToCheck(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT tm_id FROM user_artists WHERE tm_id <> ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("list artist ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan artist id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read artist ids: %w", err)
	}
	return ids, nil
}
