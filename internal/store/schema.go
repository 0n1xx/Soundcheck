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

-- The venue's time zone, to show dates in local time.
-- Added after the table was created, hence ALTER.
ALTER TABLE soundcheck.events
    ADD COLUMN IF NOT EXISTS timezone TEXT NOT NULL DEFAULT '';

-- The user's Telegram chat, set when they connect the bot.
ALTER TABLE soundcheck.users
    ADD COLUMN IF NOT EXISTS telegram_chat_id BIGINT;

-- One-time codes for connecting Telegram: the site puts the code in the
-- bot link, the bot gets it back with /start and knows who it is.
CREATE TABLE IF NOT EXISTS soundcheck.telegram_links (
    code       TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES soundcheck.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

-- Which alerts each user wants; everything is on by default.
ALTER TABLE soundcheck.users
    ADD COLUMN IF NOT EXISTS notify_new     BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS notify_24h     BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS notify_15m     BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS notify_changes BOOLEAN NOT NULL DEFAULT true;

-- Google's ID for people who sign in with Google.
ALTER TABLE soundcheck.users
    ADD COLUMN IF NOT EXISTS google_id TEXT UNIQUE;

-- Whether the user proved the email is theirs. Accounts linked to Google
-- count as verified: Google has already checked the address.
ALTER TABLE soundcheck.users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;
UPDATE soundcheck.users SET email_verified = true
    WHERE google_id IS NOT NULL AND NOT email_verified;

-- One-time links for confirming an email address. Like sessions, only a
-- hash of the token is stored. email is the address the link confirms,
-- so a link sent before an email change can't confirm the new address.
CREATE TABLE IF NOT EXISTS soundcheck.email_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES soundcheck.users(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- One-time "reset your password" links, valid for an hour. Built like
-- email_tokens: only a hash of the token, plus the address it was sent to.
CREATE TABLE IF NOT EXISTS soundcheck.password_resets (
    token_hash TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES soundcheck.users(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Alerts already sent, so nobody gets the same one twice.
-- kind is "new", "onsale_24h", "onsale_15m", "cancelled", "rescheduled"...
CREATE TABLE IF NOT EXISTS soundcheck.sent_alerts (
    user_id  BIGINT NOT NULL REFERENCES soundcheck.users(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    kind     TEXT NOT NULL,
    sent_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_id, kind)
);
`

// Migrate creates the schema and tables if they don't exist yet.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}
