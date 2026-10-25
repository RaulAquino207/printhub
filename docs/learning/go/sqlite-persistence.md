# SQLite persistence with database/sql

How the PrintHub API stores jobs in SQLite, evolves the schema safely, and finds interrupted jobs after a restart.

## 1. The problem

Until now every job lived only in memory: a restart, a crash or a power outage (the HomeLab server has had several) would lose the queue and the history. The API needs durable storage that:

- survives restarts and crashes without corrupting data;
- can evolve its schema over time without manual SQL on the server;
- keeps a job's history meaningful even if the printer configuration changes later;
- lets the worker find jobs that were interrupted mid-way.

## 2. The theory

### database/sql

Go's standard `database/sql` package is a driver-agnostic API; a driver registers itself by name in an `init` function, which is why it is imported only for its side effect (`_ "modernc.org/sqlite"`).

- `*sql.DB` is **not one connection** but a pool, safe for concurrent use. Create it once and share it.
- `QueryRowContext(...).Scan(...)` reads one row; no row means `sql.ErrNoRows`.
- `QueryContext` returns `*sql.Rows`, which holds a connection until `rows.Close()` — always close them, and check `rows.Err()` after the loop.
- `BeginTx` starts a transaction; `defer tx.Rollback()` is the idiom, because `Rollback` after a successful `Commit` is a no-op.
- `?` placeholders send values separately from SQL text, preventing SQL injection.

### embed

`//go:embed migrations/*.sql` compiles the files into the binary as an `embed.FS`. The deployed binary carries its own schema; nothing has to be copied next to it.

### SQLite concurrency and durability

- SQLite allows **many readers but a single writer** at a time, coordinated with file locks.
- **Journal mode WAL** (write-ahead log): changes are appended to a `-wal` file and later *checkpointed* into the main file. Readers keep reading the last committed state while a write happens, and a crash can never leave a half-applied transaction — on the next open, SQLite simply ignores uncommitted log frames.
- **`busy_timeout`**: when the lock is held, wait up to N ms and retry instead of failing immediately with `SQLITE_BUSY`.
- **`synchronous`**: how often SQLite forces data to disk with `fsync`. With WAL, `NORMAL` is safe against application crashes; only an OS crash or power loss can lose the last few commits (the database stays consistent). `FULL` syncs every commit, at a cost.
- **`foreign_keys`**: SQLite ignores `REFERENCES` constraints unless this is enabled — per connection.
- **`BEGIN IMMEDIATE`** takes the write lock when the transaction starts. With the default `DEFERRED`, two transactions can both read and then both try to upgrade to write, and one fails with `SQLITE_BUSY` even with a timeout.
- **`STRICT` tables** reject values of the wrong type (by default SQLite would store text in an INTEGER column).

### Migrations

A migration is a numbered, immutable script. The database records which versions it has applied; on startup, only newer ones run, in order. Because SQLite DDL is transactional, each script plus its bookkeeping row can be applied atomically.

## 3. The chosen solution

| Setting | Value | Why |
|---|---|---|
| Driver | `modernc.org/sqlite` | Pure Go (SQLite transpiled to Go): no C compiler, trivial cross-compilation and small Docker images |
| `journal_mode` | `WAL` | Crash-safe, readers do not block the writer |
| `busy_timeout` | `5000` ms | Absorb short lock waits (e.g. someone inspecting the file with the `sqlite3` CLI) |
| `synchronous` | `NORMAL` | Durable against process crashes; adequate for a print queue |
| `foreign_keys` | on | `job_transitions` rows cannot point to missing jobs |
| `_txlock` | `immediate` | No lock-upgrade failures between transactions |
| Pool | `SetMaxOpenConns(1)` | One connection serializes all access: no contention, no `SQLITE_BUSY` inside the process |
| Location | `PRINTHUB_DATA_DIR` (default `data/`), created with `0700` | Outside any web root; a persistent volume in containers |

**Single connection trade-off**: simple and contention-free, but a slow query delays everything, and code must never start a query while another's rows are still open in the same flow (it would wait forever for the only connection). `ListUnfinished` closes its rows before `loadHistory` runs for exactly this reason. If load ever grows, the next step is two pools: one writer connection plus several read-only connections.

**Schema** (`migrations/0001_create_jobs.sql`):

- `jobs` stores a **snapshot** of the printer (ID, name, model, simulated flag) and every option as columns. After the first save, only lifecycle fields change (status, provider job ID, failure reason, `updated_at`), so history stays accurate even if a printer is renamed or removed.
- `job_transitions` has one row per status the job entered, keyed by `(job_id, seq)`. Saving inserts with `ON CONFLICT DO NOTHING`, so saving the same job repeatedly only appends what is new.
- Times are RFC 3339 text in UTC: sortable, readable in the CLI, and free of Go's monotonic clock reading.

**Migration runner** (`migrate.go`):

1. Load `NNNN_description.sql` files from the embedded FS, sort by number, and refuse gaps, duplicates or badly named files before touching the database.
2. Read the current version from `schema_migrations`; refuse to start if the database is **newer** than the binary (protects against running an older build on upgraded data).
3. Apply each pending migration in its own transaction together with its `schema_migrations` row — all or nothing, exactly once.

**Stable printer IDs**: stored jobs reference printers by ID, so the simulated printers' IDs are pinned by a test (`TestDemoIDsAreStable`). Changing one would orphan existing jobs and must be treated as a data migration.

### Recovering after a restart

`Store.ListUnfinished` returns every job in a non-terminal status (`queued`, `processing`, `submitted`, `printing`), oldest first, with full history. Today the API only logs how many it found; the worker (milestone 8) will reconcile them with this policy:

| Status at restart | What we know | Planned action |
|---|---|---|
| `queued` | The provider never saw it | Put it back in the queue |
| `processing` | The document may or may not have reached the provider, and there is no provider job ID to ask about | Mark `failed` ("interrupted during submission"): printing twice is worse than asking the user to resubmit |
| `submitted`, `printing` | The provider job ID is stored | Ask the provider for that job's status and continue from there |

This is why the job is saved **after every transition**: the database always reflects the last known state.

## 4. Example from our project

Opening the database (`internal/store/store.go`):

```go
params := url.Values{
	"_journal_mode": {"WAL"},
	"_busy_timeout": {"5000"},
	"_foreign_keys": {"1"},
	"_synchronous":  {"NORMAL"},
	"_txlock":       {"immediate"},
}
db, err := sql.Open("sqlite", path+"?"+params.Encode())
db.SetMaxOpenConns(1)
```

Translating "no row" into a domain error (`internal/store/jobs.go`):

```go
j, err := scanJob(row)
if errors.Is(err, sql.ErrNoRows) {
	return nil, ErrJobNotFound
}
```

`scanJob` accepts a tiny interface, so the same code scans a `*sql.Row` and a `*sql.Rows`:

```go
type scanner interface{ Scan(dest ...any) error }
```

The persistence test that matters most (`TestJobSurvivesReopen`): save a job after each transition, **close** the database, open the same file again and compare the whole job, history included, with `reflect.DeepEqual`.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| `mattn/go-sqlite3` | The most used driver and a bit faster, but requires cgo and a C toolchain for every build target |
| PostgreSQL | More capable, but another service to run, back up and secure — unnecessary for one HomeLab app |
| ORM (GORM, ent) | Less SQL to write, but hides what runs; explicit SQL is the learning goal |
| `sqlc` | Generates type-safe Go from SQL; worth it once there are many queries |
| goose / golang-migrate | Mature migration tools with down-migrations and CLIs; our runner is ~100 lines and enough for forward-only migrations |
| `DEFERRED` transactions + larger pool | More read concurrency, but needs care to avoid `SQLITE_BUSY` on lock upgrades |
| JSON column for options | Fewer columns, but options become opaque to SQL queries and constraints |

## 6. How to validate

```bash
moon run api:test        # store: reopen test, immutable snapshot, unfinished jobs, PRAGMAs, migrations
moon run api:dev         # logs "database ready" with the file path

sqlite3 apps/api/data/printhub.db 'SELECT * FROM schema_migrations; PRAGMA journal_mode;'
# 1|0001_create_jobs|<time>
# wal

# Restart the API repeatedly: schema_migrations keeps a single row per version.
```

Recovery check used during milestone 6: insert a job in `processing` directly with `sqlite3`, restart the API, and look for `unfinished jobs found ... count=1` in the log; delete the row afterwards (its transitions go with it thanks to `ON DELETE CASCADE`).
