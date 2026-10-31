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

// UserByEmail finds a user by email, or returns ErrNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (core.User, error) {
	var u core.User
	var hash *string // NULL in the database for Google sign-ins

	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, city, created_at
		 FROM users WHERE email = $1`,
		normalizeEmail(email),
	).Scan(&u.ID, &u.Email, &hash, &u.City, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return core.User{}, ErrNotFound
	}
	if err != nil {
		return core.User{}, fmt.Errorf("find user by email: %w", err)
	}

	if hash != nil {
		u.PasswordHash = *hash
	}
	return u, nil
}

// normalizeEmail makes "Vlad@Example.com " and "vlad@example.com" the same user.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
