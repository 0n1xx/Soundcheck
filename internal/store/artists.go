package store

import (
	"context"
	"fmt"
)

// ListArtists returns the artists a user follows, oldest first.
// It's used to fill the Artists page and the counters in the sidebar.
func (s *Store) ListArtists(ctx context.Context, userID int64) ([]string, error) {
	// Query is for SELECTs that can return many rows.
	// $1 is replaced by userID safely, so no SQL injection is possible.
	rows, err := s.pool.Query(ctx,
		`SELECT name FROM user_artists
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	// The rows hold a database connection; give it back to the pool
	// when this function returns, however it returns.
	defer rows.Close()

	var artists []string

	// Read the result one row at a time, like while (reader.Read()) in C#.
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan artist: %w", err)
		}
		artists = append(artists, name)
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
func (s *Store) AddArtist(ctx context.Context, userID int64, name string) error {
	// Exec is for statements that don't return rows: INSERT, UPDATE, DELETE.
	// ON CONFLICT relies on PRIMARY KEY (user_id, name) from schema.go:
	// if this pair already exists, Postgres skips the insert quietly.
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_artists (user_id, name)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id, name) DO NOTHING`,
		userID, name,
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
