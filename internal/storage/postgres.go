package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	databaseConnectAttempts = 10
	databaseRetryDelay      = 2 * time.Second
)

// NewPostgresPool creates a pool and retries the startup ping while PostgreSQL
// finishes recovery after a host or container restart.
func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	var pingErr error
	for attempt := 1; attempt <= databaseConnectAttempts; attempt++ {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			return pool, nil
		}
		if attempt == databaseConnectAttempts {
			break
		}
		timer := time.NewTimer(databaseRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			pool.Close()
			return nil, fmt.Errorf("wait for PostgreSQL: %w", ctx.Err())
		case <-timer.C:
		}
	}
	pool.Close()
	return nil, fmt.Errorf("ping PostgreSQL after %d attempts: %w", databaseConnectAttempts, pingErr)
}
