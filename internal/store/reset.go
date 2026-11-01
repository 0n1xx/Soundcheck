package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0n1xx/soundcheck/internal/core"
)

// resetLifetime is how long a "reset your password" link works. Short,
// because anyone who gets hold of the link can take over the account.
const resetLifetime = time.Hour

// CreateResetToken makes a one-time token for a password reset link.
// Older links of the user stop working. ErrTooSoon means one was sent
// less than a minute ago.
func (s *Store) CreateResetToken(ctx context.Context, userID int64, email string) (string, error) {
	var recent bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM password_resets
		     WHERE user_id = $1 AND created_at > now() - interval '1 minute')`,
		userID,
	).Scan(&recent); err != nil {
		return "", fmt.Errorf("check recent resets: %w", err)
	}
	if recent {
		return "", ErrTooSoon
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate reset token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	if _, err := s.pool.Exec(ctx,
		`DELETE FROM password_resets WHERE user_id = $1 OR expires_at < now()`,
		userID,
	); err != nil {
		return "", fmt.Errorf("delete old resets: %w", err)
	}

	// As with sessions, only the hash is stored.
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO password_resets (token_hash, user_id, email, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		hashToken(token), userID, normalizeEmail(email), time.Now().Add(resetLifetime),
	); err != nil {
		return "", fmt.Errorf("create reset token: %w", err)
	}
	return token, nil
}

// UserByResetToken returns whose reset link this is, without using it up.
// The reset page calls it to check the link before showing the form.
// ErrNotFound means the link is unknown, used or expired.
func (s *Store) UserByResetToken(ctx context.Context, token string) (core.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+`
		 FROM password_resets r
		 JOIN users u ON u.id = r.user_id AND u.email = r.email
		 WHERE r.token_hash = $1 AND r.expires_at > now()`,
		hashToken(token),
	))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return core.User{}, fmt.Errorf("find reset token: %w", err)
	}
	return u, err
}

// ResetPassword uses up a reset link and sets a new password. Clicking a
// link from the inbox proves the email is theirs, so the address becomes
// verified. Every session is ended: if someone else knew the old password,
// they're logged out. For an address that was never verified the Telegram
// chat is dropped too, since a stranger may have connected it.
func (s *Store) ResetPassword(ctx context.Context, token, passwordHash string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx,
		`WITH r AS (
		     DELETE FROM password_resets
		     WHERE token_hash = $1 AND expires_at > now()
		     RETURNING user_id, email
		 )
		 UPDATE users u
		 SET password_hash    = $2,
		     telegram_chat_id = CASE WHEN u.email_verified THEN u.telegram_chat_id ELSE NULL END,
		     email_verified   = true
		 FROM r
		 WHERE u.id = r.user_id AND u.email = r.email
		 RETURNING u.id`,
		hashToken(token), passwordHash,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("reset password: %w", err)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return 0, fmt.Errorf("end sessions after reset: %w", err)
	}
	return userID, nil
}
