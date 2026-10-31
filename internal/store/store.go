// Package store is the only place that talks to the Postgres database.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaName keeps our tables apart from other projects that share
// the same Postgres database.
const schemaName = "soundcheck"

// Store wraps a pool of database connections.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to Postgres and checks that the database answers.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// Every connection looks for tables in our schema first,
	// so queries can say "users" instead of "soundcheck.users".
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close releases all database connections.
func (s *Store) Close() {
	s.pool.Close()
}
