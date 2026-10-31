package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0n1xx/soundcheck/internal/core"
)

// SessionDuration is how long someone stays logged in.
const SessionDuration = 30 * 24 * time.Hour

// CreateSession logs a user in. It returns the token to put in the cookie.
func (s *Store) CreateSession(ctx context.Context, userID int64) (string, time.Time, error) {
	// 32 random bytes: impossible to guess.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := time.Now().Add(SessionDuration)

	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		hashToken(token), userID, expiresAt,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return token, expiresAt, nil
}

// UserBySession finds who owns a session token, or returns ErrNotFound
// if the token is unknown or expired.
func (s *Store) UserBySession(ctx context.Context, token string) (core.User, error) {
	var u core.User
	var hash *string

	err := s.pool.QueryRow(ctx,
		`SELECT u.id, u.email, u.password_hash, u.city, u.created_at
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token = $1 AND s.expires_at > now()`,
		hashToken(token),
	).Scan(&u.ID, &u.Email, &hash, &u.City, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return core.User{}, ErrNotFound
	}
	if err != nil {
		return core.User{}, fmt.Errorf("find session: %w", err)
	}

	if hash != nil {
		u.PasswordHash = *hash
	}
	return u, nil
}

// DeleteSession logs a browser out.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, hashToken(token))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// hashToken is what we store instead of the token itself, so a leaked
// database can't be used to log in as anyone.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
