package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// telegramLinkLifetime is how long a "Connect Telegram" link works.
const telegramLinkLifetime = 10 * time.Minute

// CreateTelegramLink makes a one-time code for the bot link
// t.me/<bot>?start=<code>. Older codes of this user stop working.
func (s *Store) CreateTelegramLink(ctx context.Context, userID int64) (string, error) {
	// 16 random bytes = 22 characters, fine for Telegram's start parameter
	// (letters, digits, "_" and "-", up to 64 characters).
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate telegram code: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(raw)

	// Clean up: one user, one working link at a time.
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM telegram_links WHERE user_id = $1 OR expires_at < now()`,
		userID,
	); err != nil {
		return "", fmt.Errorf("delete old telegram links: %w", err)
	}

	if _, err := s.pool.Exec(ctx,
		`INSERT INTO telegram_links (code, user_id, expires_at) VALUES ($1, $2, $3)`,
		code, userID, time.Now().Add(telegramLinkLifetime),
	); err != nil {
		return "", fmt.Errorf("create telegram link: %w", err)
	}
	return code, nil
}

// ConnectTelegram uses up a code from the bot link and saves the chat for
// its user. It returns the user's email, or ErrNotFound if the code is
// unknown, already used or expired.
func (s *Store) ConnectTelegram(ctx context.Context, code string, chatID int64) (string, error) {
	var email string
	// One statement does both steps: delete the code (so it works only once)
	// and save the chat for the user the code belonged to.
	err := s.pool.QueryRow(ctx,
		`WITH link AS (
		     DELETE FROM telegram_links
		     WHERE code = $1 AND expires_at > now()
		     RETURNING user_id
		 )
		 UPDATE users SET telegram_chat_id = $2
		 FROM link
		 WHERE users.id = link.user_id
		 RETURNING users.email`,
		code, chatID,
	).Scan(&email)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("connect telegram: %w", err)
	}
	return email, nil
}

// DisconnectTelegram stops Telegram alerts for a user.
func (s *Store) DisconnectTelegram(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET telegram_chat_id = NULL WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("disconnect telegram: %w", err)
	}
	return nil
}
