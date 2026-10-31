package store

import (
	"context"
	"fmt"
)

// schema creates everything Soundcheck needs. Every statement uses
// IF NOT EXISTS, so running it on every start is safe.
const schema = `
CREATE SCHEMA IF NOT EXISTS soundcheck;

-- People who signed up. password_hash is empty for Google sign-ins.
CREATE TABLE IF NOT EXISTS soundcheck.users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    city          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Logged-in browsers. The token lives in a cookie.
CREATE TABLE IF NOT EXISTS soundcheck.sessions (
    token      TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES soundcheck.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

-- Artists each user follows.
CREATE TABLE IF NOT EXISTS soundcheck.user_artists (
    user_id    BIGINT NOT NULL REFERENCES soundcheck.users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, name)
);

-- Ticketmaster's ID for the artist, used by the worker to find their
-- concerts precisely. Added after the table was created, hence ALTER.
ALTER TABLE soundcheck.user_artists
    ADD COLUMN IF NOT EXISTS tm_id TEXT NOT NULL DEFAULT '';


-- Concerts found by the worker. Each concert is stored once,
-- no matter how many users follow the artist.
CREATE TABLE IF NOT EXISTS soundcheck.events (
    id          TEXT PRIMARY KEY,           -- Ticketmaster's event ID
    artist_id   TEXT NOT NULL,              -- Ticketmaster's artist ID
    name        TEXT NOT NULL,
    venue       TEXT NOT NULL DEFAULT '',
    city        TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL DEFAULT '',
    starts_at   TIMESTAMPTZ,                -- NULL if the date isn't announced yet
    on_sale_at  TIMESTAMPTZ,                -- when public ticket sales start
    status      TEXT NOT NULL DEFAULT '',   -- onsale, offsale, cancelled, rescheduled...
    first_seen  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The Upcoming shows page looks events up by artist.
CREATE INDEX IF NOT EXISTS events_artist_id_idx ON soundcheck.events (artist_id);
`

// Migrate creates the schema and tables if they don't exist yet.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}
