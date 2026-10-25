// Package store persists PrintHub data in SQLite.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver (pure Go, no cgo)
)

// Store is the SQLite-backed persistence layer. It is safe for concurrent use.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database file at path, configures the
// connection and applies pending migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	// The driver runs these on every new connection.
	params := url.Values{
		// Write-ahead log: readers do not block the writer and vice versa,
		// and a crash cannot leave a half-written transaction.
		"_journal_mode": {"WAL"},
		// Wait up to 5 s for a lock instead of failing at once with SQLITE_BUSY.
		"_busy_timeout": {"5000"},
		// SQLite does not enforce REFERENCES unless asked to, per connection.
		"_foreign_keys": {"1"},
		// With WAL, NORMAL is durable against application crashes; only a
		// power loss can drop the last transactions. FULL syncs on every commit.
		"_synchronous": {"NORMAL"},
		// Take the write lock when a transaction begins, not at its first
		// write, avoiding lock-upgrade failures between transactions.
		"_txlock": {"immediate"},
	}
	db, err := sql.Open("sqlite", path+"?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	// SQLite allows a single writer. One connection serializes all access,
	// which removes lock contention entirely and is plenty for PrintHub's load.
	// Consequence: never start a query while the rows of another are still open
	// in the same flow, or it waits forever for the only connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(ctx, db, migrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}

	return &Store{db: db}, nil
}

// Close releases the database. With WAL, closing the last connection also
// checkpoints the log into the main database file.
func (s *Store) Close() error {
	return s.db.Close()
}

// Times are stored as RFC 3339 text in UTC: sortable, readable with the
// sqlite3 CLI, and free of Go's monotonic clock reading.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
