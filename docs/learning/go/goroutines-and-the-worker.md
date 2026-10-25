# Goroutines, channels and the job worker

How PrintHub processes print jobs in the background, stays consistent with concurrent cancellations, and shuts down cleanly.

## 1. The problem

Creating a job must return immediately, but printing takes seconds or minutes and can fail. Something has to submit queued jobs, follow their progress at the provider, react to cancellations arriving through HTTP at any moment, recover jobs interrupted by a restart, and stop cleanly when the process is asked to exit — without losing jobs or corrupting their state.

## 2. The theory

- **Goroutines** are functions running concurrently, scheduled by the Go runtime onto OS threads. They are cheap (a few KiB of stack), started with `go f()`.
- **Channels** carry values between goroutines and synchronize them. A **buffered** channel (`make(chan T, n)`) accepts up to `n` values without a receiver.
- **`select`** waits on several channel operations and runs whichever is ready first; a `default` case makes it non-blocking.
- **`context.Context`** propagates cancellation: when `cancel()` is called, `ctx.Done()` closes and every goroutine selecting on it can stop.
- **`sync.Mutex`** protects data touched by several goroutines; only one holder at a time.
- **`sync.WaitGroup`** waits for goroutines to finish. `wg.Go(f)` (Go 1.25+) starts `f` and tracks it.
- **The race detector** (`go test -race`) instruments memory accesses and reports unsynchronized concurrent access at runtime.
- NestJS comparison: there is no event loop to lean on. Background work is an explicit goroutine you start, coordinate and stop yourself — closer to a hand-written job runner than to `@nestjs/schedule` or BullMQ.

## 3. The chosen solution

**One worker goroutine, the database as source of truth.** Every *pass* reads the unfinished jobs from SQLite (oldest first) and advances each by one step:

| Job status | Step |
|---|---|
| `queued` | Save `processing` → check the printer → open the document → `Submit` to the provider → save `submitted` with the provider job ID |
| `processing` | Only possible after a crash (submission happens entirely under the lock) → `failed: interrupted` |
| `submitted` / `printing` | Ask the provider: pending → wait; processing → `printing`; completed → `completed`; failed → `failed: printer_error`; cancelled outside PrintHub → `cancelled`; unknown job → `failed: provider_lost_job` |

Finished jobs have their document deleted. Failure reasons are stable codes (`printer_unavailable`, `document_missing`, `submit_failed`, `interrupted`, `provider_lost_job`, `printer_error`).

**Waking up.** The loop runs a pass, then `select`s on three channels: the context (stop), a ticker (every second), and a *wake* channel that `POST /jobs` signals through `Notify`. The wake channel has capacity 1 and `Notify` sends with `select … default`, so it never blocks: if a signal is already pending, another adds nothing. Because each pass reads the database, a missed signal only delays a job until the next tick — it can never lose it.

**Restart recovery is not special code**: it is the first pass, which finds whatever the previous run left behind.

**Concurrent cancellation.** `Cancel` runs in HTTP handler goroutines while the worker runs in its own. A mutex makes each job step (load → change → save) atomic with respect to `Cancel`: the worker reloads the job under the lock, and `Cancel` waits for an in-flight submission to finish, then cancels at the provider before recording `cancelled`. If the provider already finished the job, cancelling is refused (`409`) and the next pass records the real outcome.

**Shutdown order** (`main.go`):

1. The signal context is cancelled (Ctrl+C, SIGTERM).
2. `srv.Shutdown` stops accepting connections and drains in-flight requests — which may still call `Cancel`.
3. The worker's own context is cancelled; `wg.Wait()` waits for its current step.
4. The database closes last (deferred calls run in reverse order).

A submission interrupted by shutdown stays `processing` and is failed as `interrupted` on the next start.

**Orphan documents** (an upload stored but its job never saved, e.g. a crash in between) are removed by `CleanDocuments` before the HTTP server starts, so it never races with an upload.

**The mock provider** keeps no goroutines: a job's state is computed from the time elapsed since submission, behind a mutex, with an injectable clock. Demo timing: 2 s pending, 5 s printing. "Laser com defeito" fails every job.

## 4. Example from our project

The loop (`internal/worker/worker.go`):

```go
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
		w.pass(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}
```

A send that never blocks:

```go
func (w *Worker) Notify() {
	select {
	case w.wake <- struct{}{}:
	default: // a pass is already pending
	}
}
```

Starting and stopping it (`cmd/printhub-api/main.go`):

```go
workerCtx, stopWorker := context.WithCancel(context.Background())
var wg sync.WaitGroup
wg.Go(func() { jobs.Run(workerCtx) })
defer func() {
	stopWorker()
	wg.Wait()
}()
```

Tests drive the worker deterministically: real SQLite, real document storage and the real mock with a fake clock, calling `pass` directly and moving time forward — no `sleep` except in the one test that checks `Run` itself reacts to `Notify` and to cancellation.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| One goroutine per job | More parallelism, but jobs for the same printer would race, and coordination gets harder; one printer prints one job at a time anyway |
| A pool of N workers | Useful with many printers; adds per-job locking. Easy to add later because the database already holds the queue |
| In-memory channel as the queue | Fast, but jobs are lost on restart and a full channel blocks producers |
| External queue (Redis, RabbitMQ) | Explicitly out of scope: another service to run, no benefit at this scale |
| Push from the provider (CUPS notifications) | Fewer polls, but more moving parts; polling every second is cheap here |
| Retrying failed submissions automatically | Risk of duplicate prints; the user resubmits instead (retry is a future feature) |

## 6. How to validate

```bash
go -C apps/api test -race ./internal/worker ./internal/printing/mock

OPTS='{"copies":1,"colorMode":"monochrome","paperSize":"A4","orientation":"portrait","duplex":false,"pageRanges":""}'
ID=$(curl -s -X POST localhost:8080/api/v1/jobs -F printerId=mono-laser -F "options=$OPTS" -F file=@teste.pdf | jq -r .id)
watch -n1 "curl -s localhost:8080/api/v1/jobs/$ID | jq -r '.status'"
# queued → submitted → printing → completed in about 7 s; the document disappears from data/documents

# printerId=faulty-laser          → failed, printer_error
# POST /api/v1/jobs/$ID/cancel    → cancelled (or 409 once finished)
# restart the API while a job is submitted → failed, provider_lost_job (the mock forgot it)
```
