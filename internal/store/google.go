package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/0n1xx/soundcheck/internal/core"
)

// SignInWithGoogle returns the user for a Google account, looking in order:
//
//  1. someone already signed in with this Google account: that user;
//  2. someone signed up with the same email and a password: Google is
//     linked to their account (Google has checked the email is theirs).
//     If that email was never verified, whoever created the account may
//     not own the address, so their password, sessions and Telegram link
//     are removed: the account now belongs to the real owner;
//  3. nobody: a new user without a password.
//
// created is true only in case 3. ErrEmailTaken means the email belongs to
// a user who is linked to a different Google account.
func (s *Store) SignInWithGoogle(ctx context.Context, googleID, email string) (core.User, bool, error) {
	email = normalizeEmail(email)

	// 1. Known Google account.
	u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users u WHERE u.google_id = $1`,
		googleID,
	))
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return core.User{}, false, fmt.Errorf("find google user: %w", err)
	}

	// 2. Existing account with this email: link it.
	//    First log out everyone on an unverified account with this email,
	//    so a stranger who registered the address can't stay logged in.
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM sessions WHERE user_id IN (
		     SELECT id FROM users
		     WHERE email = $1 AND google_id IS NULL AND NOT email_verified)`,
		email,
	); err != nil {
		return core.User{}, false, fmt.Errorf("end sessions before linking: %w", err)
	}

	//    Then link Google. For an unverified account the password and the
	//    Telegram chat were set by someone unknown, so they're dropped.
	u, err = scanUser(s.pool.QueryRow(ctx,
		`UPDATE users u
		 SET google_id = $1,
		     password_hash    = CASE WHEN u.email_verified THEN u.password_hash ELSE NULL END,
		     telegram_chat_id = CASE WHEN u.email_verified THEN u.telegram_chat_id ELSE NULL END,
		     email_verified   = true
		 WHERE u.email = $2 AND u.google_id IS NULL
		 RETURNING `+userColumns,
		googleID, email,
	))
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return core.User{}, false, fmt.Errorf("link google account: %w", err)
	}

	// 3. New user. password_hash stays NULL: they sign in with Google.
	u, err = scanUser(s.pool.QueryRow(ctx,
		`INSERT INTO users AS u (email, google_id, email_verified) VALUES ($1, $2, true)
		 RETURNING `+userColumns,
		email, googleID,
	))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return core.User{}, false, ErrEmailTaken
	}
	if err != nil {
		return core.User{}, false, fmt.Errorf("create google user: %w", err)
	}
	return u, true, nil
}
