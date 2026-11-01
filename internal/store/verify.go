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

// emailTokenLifetime is how long a confirmation link works.
const emailTokenLifetime = 24 * time.Hour

// ErrTooSoon means a confirmation email was sent less than a minute ago.
// It stops anyone from using the "Resend" button to flood an inbox.
var ErrTooSoon = errors.New("a confirmation email was sent a moment ago")

// CreateEmailToken makes a one-time token for the link that confirms email
// for this user. Older tokens of the user stop working.
func (s *Store) CreateEmailToken(ctx context.Context, userID int64, email string) (string, error) {
	var recent bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM email_tokens
		     WHERE user_id = $1 AND created_at > now() - interval '1 minute')`,
		userID,
	).Scan(&recent); err != nil {
		return "", fmt.Errorf("check recent email tokens: %w", err)
	}
	if recent {
		return "", ErrTooSoon
	}

	// 32 random bytes, like a session token: impossible to guess.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate email token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	// Clean up: one working link per user, and drop expired ones.
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM email_tokens WHERE user_id = $1 OR expires_at < now()`,
		userID,
	); err != nil {
		return "", fmt.Errorf("delete old email tokens: %w", err)
	}

	// Only the hash is stored, so a leaked database can't confirm anything.
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO email_tokens (token_hash, user_id, email, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		hashToken(token), userID, normalizeEmail(email), time.Now().Add(emailTokenLifetime),
	); err != nil {
		return "", fmt.Errorf("create email token: %w", err)
	}
	return token, nil
}

// VerifyEmail uses up a token from a confirmation link, marks the email as
// verified and returns whose it was. It returns ErrNotFound if the token is
// unknown, used, expired, or was sent to an address the user has since changed.
func (s *Store) VerifyEmail(ctx context.Context, token string) (int64, error) {
	// Same trick as ConnectTelegram: delete the token and use it in one
	// statement, so it can only ever work once.
	var userID int64
	err := s.pool.QueryRow(ctx,
		`WITH t AS (
		     DELETE FROM email_tokens
		     WHERE token_hash = $1 AND expires_at > now()
		     RETURNING user_id, email
		 )
		 UPDATE users SET email_verified = true
		 FROM t
		 WHERE users.id = t.user_id AND users.email = t.email
		 RETURNING users.id`,
		hashToken(token),
	).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("verify email: %w", err)
	}
	return userID, nil
}
