package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

// migrationFiles holds the SQL migrations, compiled into the binary.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migration is one versioned schema change, loaded from a file named
// NNNN_description.sql.
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads every .sql file in fsys and returns them sorted by
// version. Versions must be unique and contiguous from 1, so a missing or
// duplicated file is caught before anything is applied.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}

	var migrations []migration
	for _, file := range files {
		prefix, _, ok := strings.Cut(file, "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil || version < 1 {
			return nil, fmt.Errorf("migration %q: name must look like 0001_description.sql", file)
		}
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("migration %q: %w", file, err)
		}
		migrations = append(migrations, migration{
			version: version,
			name:    strings.TrimSuffix(path.Base(file), ".sql"),
			sql:     string(content),
		})
	}

	slices.SortFunc(migrations, func(a, b migration) int { return a.version - b.version })
	for i, m := range migrations {
		if m.version != i+1 {
			return nil, fmt.Errorf("migration %q: expected version %d (versions must be unique and contiguous)", m.name, i+1)
		}
	}
	return migrations, nil
}

// migrate applies, in order, every migration newer than the database's
// current version. Each migration runs in its own transaction together with
// the row that records it, so it is applied completely and exactly once, or
// not at all (SQLite DDL is transactional).
func migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	migrations, err := loadMigrations(fsys)
	if err != nil {
		return err
	}

	const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT    NOT NULL,
		applied_at TEXT    NOT NULL
	) STRICT`
	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than this binary supports (%d)", current, len(migrations))
	}

	for _, m := range migrations[current:] {
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %s: %w", m.name, err)
	}
	// Rollback after a successful Commit is a no-op, so deferring it covers
	// every early return.
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("migration %s: %w", m.name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		m.version, m.name, formatTime(time.Now()),
	); err != nil {
		return fmt.Errorf("migration %s: record version: %w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %s: commit: %w", m.name, err)
	}
	return nil
}
