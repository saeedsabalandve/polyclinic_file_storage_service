package repository

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// NewDBPool creates a new database connection pool
func NewDBPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("unable to parse database URL: %w", err)
    }

    // Configure pool settings
    config.MaxConns = 25
    config.MinConns = 5
    config.MaxConnLifetime = 3600 // 1 hour in seconds
    config.MaxConnIdleTime = 300  // 5 minutes in seconds

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("unable to create connection pool: %w", err)
    }

    // Test the connection
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, fmt.Errorf("unable to ping database: %w", err)
    }

    return pool, nil
}
