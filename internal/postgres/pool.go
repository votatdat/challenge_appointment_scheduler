package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string, maxConnections int32, connectTimeout time.Duration) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	poolConfig.MaxConns = maxConnections
	// Pool connection attempts can outlive the acquiring request. Bound their
	// handshake too, including background replenishment and startup cleanup.
	poolConfig.ConnConfig.ConnectTimeout = connectTimeout

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
