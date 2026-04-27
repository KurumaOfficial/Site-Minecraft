// Автор: Kuruma
package database

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"holo-site-backend/internal/config"
)

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func RunSchema(ctx context.Context, pool *pgxpool.Pool, schemaPath string) error {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, string(content))
	return err
}
