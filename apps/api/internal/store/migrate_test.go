package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

// openRawDB opens an empty database without running the real migrations, so
// tests can apply their own.
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func appliedVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	return versions
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`, name).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestMigrateAppliesInOrderOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db := openRawDB(t)

	// 0002 depends on 0001; the files are listed out of order on purpose.
	fsys := fstest.MapFS{
		"0002_add_b.sql": {Data: []byte(`CREATE TABLE b (a_id INTEGER REFERENCES a (id))`)},
		"0001_add_a.sql": {Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY)`)},
	}

	if err := migrate(ctx, db, fsys); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if got := appliedVersions(t, db); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("versions = %v, want [1 2]", got)
	}

	// Running again must not re-apply anything (CREATE TABLE would fail).
	if err := migrate(ctx, db, fsys); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	// A new migration is applied on its own.
	fsys["0003_add_c.sql"] = &fstest.MapFile{Data: []byte(`CREATE TABLE c (id INTEGER)`)}
	if err := migrate(ctx, db, fsys); err != nil {
		t.Fatalf("third migrate: %v", err)
	}
	if got := appliedVersions(t, db); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("versions = %v, want [1 2 3]", got)
	}
}

func TestMigrateFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	db := openRawDB(t)

	fsys := fstest.MapFS{
		"0001_ok.sql": {Data: []byte(`CREATE TABLE a (id INTEGER)`)},
		// The first statement succeeds, the second fails: neither may persist.
		"0002_broken.sql": {Data: []byte(`CREATE TABLE b (id INTEGER); CREATE TABLE a (id INTEGER);`)},
	}

	if err := migrate(ctx, db, fsys); err == nil {
		t.Fatal("migrate succeeded, want an error from 0002")
	}
	if tableExists(t, db, "b") {
		t.Error("table b exists: the failed migration was not rolled back")
	}
	if got := appliedVersions(t, db); !slices.Equal(got, []int{1}) {
		t.Errorf("versions = %v, want [1]", got)
	}
}

func TestMigrateRejectsInvalidSets(t *testing.T) {
	tests := []struct {
		name string
		fsys fstest.MapFS
		want string
	}{
		{
			name: "gap",
			fsys: fstest.MapFS{"0001_a.sql": {}, "0003_c.sql": {}},
			want: "expected version 2",
		},
		{
			name: "duplicate",
			fsys: fstest.MapFS{"0001_a.sql": {}, "0001_b.sql": {}},
			want: "expected version 2",
		},
		{
			name: "bad name",
			fsys: fstest.MapFS{"init.sql": {}},
			want: "name must look like",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := migrate(context.Background(), openRawDB(t), tt.fsys)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestMigrateRejectsNewerDatabase(t *testing.T) {
	ctx := context.Background()
	db := openRawDB(t)
	two := fstest.MapFS{
		"0001_a.sql": {Data: []byte(`CREATE TABLE a (id INTEGER)`)},
		"0002_b.sql": {Data: []byte(`CREATE TABLE b (id INTEGER)`)},
	}
	if err := migrate(ctx, db, two); err != nil {
		t.Fatal(err)
	}

	// An older binary that only knows migration 1 must refuse to run.
	delete(two, "0002_b.sql")
	if err := migrate(ctx, db, two); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("err = %v, want a 'newer than this binary' error", err)
	}
}

func TestEmbeddedMigrationsAreValid(t *testing.T) {
	if _, err := loadMigrations(mustSub(t)); err != nil {
		t.Fatal(err)
	}
}
