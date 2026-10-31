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
`

// Migrate creates the schema and tables if they don't exist yet.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}
