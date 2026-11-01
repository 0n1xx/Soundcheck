package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/0n1xx/soundcheck/internal/core"
)

// Errors the website can check for and show a friendly message.
var (
	ErrEmailTaken = errors.New("email is already registered")
	ErrNotFound   = errors.New("not found")
)

// CreateUser saves a new user and returns it with its new ID.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (core.User, error) {
	u := core.User{
		Email:        normalizeEmail(email),
		PasswordHash: passwordHash,
	}

	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, created_at`,
		u.Email, u.PasswordHash,
	).Scan(&u.ID, &u.CreatedAt)

	// 23505 is Postgres' code for "unique value already exists".
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return core.User{}, ErrEmailTaken
	}
	if err != nil {
		return core.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// userColumns is every column of a user, in the order scanUser reads them.
// Queries that load a user call the users table "u".
const userColumns = `u.id, u.email, u.password_hash, u.city, u.created_at,
	COALESCE(u.telegram_chat_id, 0),
	u.notify_new, u.notify_24h, u.notify_15m, u.notify_changes`

// scanUser reads one row selected with userColumns.
func scanUser(row pgx.Row) (core.User, error) {
	var u core.User
	var hash *string // NULL in the database for Google sign-ins

	err := row.Scan(&u.ID, &u.Email, &hash, &u.City, &u.CreatedAt,
		&u.TelegramChatID,
		&u.NotifyNew, &u.Notify24h, &u.Notify15m, &u.NotifyChanges)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.User{}, ErrNotFound
	}
	if err != nil {
		return core.User{}, err
	}

	if hash != nil {
		u.PasswordHash = *hash
	}
	return u, nil
}

// UserByEmail finds a user by email, or returns ErrNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (core.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users u WHERE u.email = $1`,
		normalizeEmail(email),
	))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return core.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return u, err
}

// normalizeEmail makes "Vlad@Example.com " and "vlad@example.com" the same user.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
