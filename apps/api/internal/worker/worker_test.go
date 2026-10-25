package worker

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/document"
	"github.com/RaulAquino207/printhub/apps/api/internal/job"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing/mock"
	"github.com/RaulAquino207/printhub/apps/api/internal/store"
)

// clock is a test clock, safe to read from the worker goroutine.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	w     *Worker
	store *store.Store
	docs  *document.Storage
	mock  *mock.Provider
	clock *clock
}

// newEnv wires the worker to real components: SQLite, document storage and
// the mock provider, with a controllable clock.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()

	st, err := store.Open(ctx, filepath.Join(dir, "printhub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	docs, err := document.NewStorage(filepath.Join(dir, "documents"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	c := &clock{t: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)}
	provider := mock.New(
		mock.Printer{Destination: "q-ok", Status: printer.StatusOnline},
		mock.Printer{Destination: "q-jam", Status: printer.StatusOnline, FailJobs: true},
		mock.Printer{Destination: "q-off", Status: printer.StatusOffline},
	)
	provider.SetClock(c.now)
	provider.SetTiming(mock.Timing{Queue: time.Second, Print: 2 * time.Second})

	printers := printer.NewRegistry([]printer.Config{
		{ID: "ok", Destination: "q-ok"},
		{ID: "jam", Destination: "q-jam"},
		{ID: "off", Destination: "q-off"},
	}, provider)

	w := New(Config{
		Store:     st,
		Printers:  printers,
		Provider:  provider,
		Documents: docs,
		Logger:    slog.New(slog.DiscardHandler),
		Interval:  time.Hour, // tests drive passes explicitly
		Now:       c.now,
	})
	return &env{w: w, store: st, docs: docs, mock: provider, clock: c}
}

// queue stores a document and a queued job for the given printer.
func (e *env) queue(t *testing.T, id job.ID, printerID printer.ID) {
	t.Helper()
	if _, err := e.docs.Save(string(id), strings.NewReader("%PDF-1.7 test")); err != nil {
		t.Fatal(err)
	}
	j := job.New(id, job.PrinterSnapshot{ID: printerID}, job.Document{Name: "a.pdf"}, printer.Options{Copies: 1}, e.clock.now())
	if err := e.store.SaveJob(context.Background(), j); err != nil {
		t.Fatal(err)
	}
}

func (e *env) job(t *testing.T, id job.ID) *job.Job {
	t.Helper()
	j, err := e.store.GetJob(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (e *env) hasDocument(t *testing.T, id job.ID) bool {
	t.Helper()
	ids, err := e.docs.IDs()
	if err != nil {
		t.Fatal(err)
	}
	return slices.Contains(ids, string(id))
}

func statuses(j *job.Job) []job.Status {
	var s []job.Status
	for _, tr := range j.History {
		s = append(s, tr.Status)
	}
	return s
}

func TestJobRunsToCompletion(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.queue(t, "j1", "ok")

	e.w.pass(ctx)
	if j := e.job(t, "j1"); j.Status != job.StatusSubmitted || j.ProviderJobID == "" {
		t.Fatalf("after first pass: %s (provider id %q), want submitted", j.Status, j.ProviderJobID)
	}
	if !e.hasDocument(t, "j1") {
		t.Fatal("document removed while the job is still active")
	}

	e.clock.advance(time.Second) // the provider starts printing
	e.w.pass(ctx)
	if got := e.job(t, "j1").Status; got != job.StatusPrinting {
		t.Fatalf("status = %s, want printing", got)
	}

	e.clock.advance(2 * time.Second) // the provider finishes
	e.w.pass(ctx)

	j := e.job(t, "j1")
	want := []job.Status{job.StatusQueued, job.StatusProcessing, job.StatusSubmitted, job.StatusPrinting, job.StatusCompleted}
	if !slices.Equal(statuses(j), want) {
		t.Errorf("history = %v, want %v", statuses(j), want)
	}
	if e.hasDocument(t, "j1") {
		t.Error("document kept after the job finished")
	}
}

func TestFailures(t *testing.T) {
	tests := []struct {
		name    string
		printer printer.ID
		setup   func(e *env)
		want    string
	}{
		{name: "printer reports failure", printer: "jam", want: ReasonPrinterError},
		{name: "printer offline", printer: "off", want: ReasonPrinterUnavailable},
		{name: "printer no longer configured", printer: "gone", want: ReasonPrinterUnavailable},
		{
			name:    "document missing",
			printer: "ok",
			setup:   func(e *env) { e.docs.Remove("j1") },
			want:    ReasonDocumentMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.queue(t, "j1", tt.printer)
			if tt.setup != nil {
				tt.setup(e)
			}

			// Enough passes and time for any job to finish.
			for range 3 {
				e.w.pass(context.Background())
				e.clock.advance(time.Second)
			}

			j := e.job(t, "j1")
			if j.Status != job.StatusFailed || j.FailureReason != tt.want {
				t.Errorf("job = %s %q, want failed %q", j.Status, j.FailureReason, tt.want)
			}
			if e.hasDocument(t, "j1") {
				t.Error("document kept after the job failed")
			}
		})
	}
}

func TestRecoveryAfterRestart(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	// A job left in processing by a crash.
	e.queue(t, "crashed", "ok")
	crashed := e.job(t, "crashed")
	crashed.StartProcessing(e.clock.now())
	e.store.SaveJob(ctx, crashed)

	// A job submitted to a provider that has since restarted and lost it.
	e.queue(t, "lost", "ok")
	lost := e.job(t, "lost")
	lost.StartProcessing(e.clock.now())
	lost.MarkSubmitted("mock-from-a-previous-run", e.clock.now())
	e.store.SaveJob(ctx, lost)

	// A queued job simply continues.
	e.queue(t, "waiting", "ok")

	e.w.pass(ctx)

	if j := e.job(t, "crashed"); j.Status != job.StatusFailed || j.FailureReason != ReasonInterrupted {
		t.Errorf("crashed = %s %q, want failed interrupted", j.Status, j.FailureReason)
	}
	if j := e.job(t, "lost"); j.Status != job.StatusFailed || j.FailureReason != ReasonProviderLostJob {
		t.Errorf("lost = %s %q, want failed provider_lost_job", j.Status, j.FailureReason)
	}
	if j := e.job(t, "waiting"); j.Status != job.StatusSubmitted {
		t.Errorf("waiting = %s, want submitted", j.Status)
	}
}

func TestCancel(t *testing.T) {
	ctx := context.Background()

	t.Run("queued", func(t *testing.T) {
		e := newEnv(t)
		e.queue(t, "j1", "ok")

		j, err := e.w.Cancel(ctx, "j1")
		if err != nil || j.Status != job.StatusCancelled {
			t.Fatalf("Cancel = %v, %v; want cancelled", j, err)
		}
		if e.hasDocument(t, "j1") {
			t.Error("document kept after cancel")
		}
		// The worker must not resurrect it.
		e.w.pass(ctx)
		if got := e.job(t, "j1").Status; got != job.StatusCancelled {
			t.Errorf("status after pass = %s, want cancelled", got)
		}
	})

	t.Run("submitted, also cancelled at the provider", func(t *testing.T) {
		e := newEnv(t)
		e.queue(t, "j1", "ok")
		e.w.pass(ctx)

		if _, err := e.w.Cancel(ctx, "j1"); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		st, err := e.mock.JobStatus(ctx, e.job(t, "j1").ProviderJobID)
		if err != nil || st.State != "cancelled" {
			t.Errorf("provider state = %+v, %v; want cancelled", st, err)
		}
	})

	t.Run("finished at the provider", func(t *testing.T) {
		e := newEnv(t)
		e.queue(t, "j1", "ok")
		e.w.pass(ctx)
		e.clock.advance(time.Hour) // finished, but PrintHub has not polled yet

		if _, err := e.w.Cancel(ctx, "j1"); !errors.Is(err, ErrNotCancellable) {
			t.Fatalf("err = %v, want ErrNotCancellable", err)
		}
		e.w.pass(ctx)
		if got := e.job(t, "j1").Status; got != job.StatusCompleted {
			t.Errorf("status = %s, want completed", got)
		}
	})

	t.Run("terminal", func(t *testing.T) {
		e := newEnv(t)
		e.queue(t, "j1", "off")
		e.w.pass(ctx) // fails: printer offline

		_, err := e.w.Cancel(ctx, "j1")
		if !errors.Is(err, ErrNotCancellable) || !errors.Is(err, job.ErrInvalidTransition) {
			t.Errorf("err = %v, want ErrNotCancellable wrapping ErrInvalidTransition", err)
		}
	})

	t.Run("unknown job", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.w.Cancel(ctx, "nope"); !errors.Is(err, job.ErrNotFound) {
			t.Errorf("err = %v, want job.ErrNotFound", err)
		}
	})
}

func TestCleanDocuments(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	e.queue(t, "active", "ok")
	e.docs.Save("orphan", strings.NewReader("%PDF-orphan")) // no job
	e.queue(t, "done", "ok")
	done := e.job(t, "done")
	done.Cancel(e.clock.now())
	e.store.SaveJob(ctx, done) // terminal, document left behind

	if err := e.w.CleanDocuments(ctx); err != nil {
		t.Fatal(err)
	}

	ids, _ := e.docs.IDs()
	if !slices.Equal(ids, []string{"active"}) {
		t.Errorf("documents = %v, want [active]", ids)
	}
}

func TestRunWakesOnNotifyAndStopsOnCancel(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		e.w.Run(ctx) // first pass happens immediately; then waits (interval: 1h)
		close(done)
	}()

	// Queue a job after Run started, then wake the worker.
	time.Sleep(50 * time.Millisecond)
	e.queue(t, "j1", "ok")
	e.w.Notify()

	deadline := time.Now().Add(2 * time.Second)
	for e.job(t, "j1").Status != job.StatusSubmitted {
		if time.Now().After(deadline) {
			t.Fatal("job not submitted after Notify")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

func TestNotifyNeverBlocks(t *testing.T) {
	e := newEnv(t)
	for range 100 {
		e.w.Notify() // nobody is receiving: must not block
	}
}
