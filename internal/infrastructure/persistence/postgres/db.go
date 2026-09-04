package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, dsn string, maxOpenConns int, maxIdleConns int, connMaxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	migrationPaths, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("read postgres migrations: %w", err)
	}

	for _, migrationPath := range migrationPaths {
		statement, err := migrationFiles.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read postgres migration %s: %w", migrationPath, err)
		}

		if _, err := db.ExecContext(ctx, string(statement)); err != nil {
			return fmt.Errorf("apply postgres migration %s: %w", migrationPath, err)
		}
	}

	return nil
}
