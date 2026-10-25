// Package worker moves print jobs through their lifecycle in the background:
// it submits queued jobs to the printing provider, follows them until they
// finish, recovers jobs interrupted by a restart and removes documents that
// are no longer needed.
package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/job"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing"
)

// Store persists jobs. *store.Store satisfies it.
type Store interface {
	SaveJob(ctx context.Context, j *job.Job) error
	GetJob(ctx context.Context, id job.ID) (*job.Job, error)
	ListUnfinished(ctx context.Context) ([]*job.Job, error)
}

// Printers looks up the current configuration of a printer.
type Printers interface {
	Get(ctx context.Context, id printer.ID) (printer.Printer, error)
}

// Provider runs jobs on printers. *mock.Provider satisfies it.
type Provider interface {
	Submit(ctx context.Context, destination string, doc io.Reader, opts printer.Options) (string, error)
	JobStatus(ctx context.Context, providerJobID string) (printing.JobStatus, error)
	Cancel(ctx context.Context, providerJobID string) error
}

// Documents gives access to uploaded files. *document.Storage satisfies it.
type Documents interface {
	Open(id string) (io.ReadCloser, error)
	Remove(id string) error
	IDs() ([]string, error)
}

// Failure reasons stored in job.FailureReason. They are stable codes; the web
// maps them to pt-BR messages.
const (
	ReasonPrinterUnavailable = "printer_unavailable" // offline or no longer configured
	ReasonDocumentMissing    = "document_missing"    // the uploaded file is gone
	ReasonSubmitFailed       = "submit_failed"       // the provider refused the job
	ReasonInterrupted        = "interrupted"         // PrintHub stopped while submitting
	ReasonProviderLostJob    = "provider_lost_job"   // the provider no longer knows the job
	ReasonPrinterError       = "printer_error"       // the provider reported a failure
)

// Config holds the worker's dependencies and settings.
type Config struct {
	Store     Store
	Printers  Printers
	Provider  Provider
	Documents Documents
	Logger    *slog.Logger
	// Interval is how often the worker checks jobs when nothing wakes it.
	Interval time.Duration
	// Now returns the current time; defaults to time.Now.
	Now func() time.Time
}

// Worker processes jobs one at a time in a single goroutine. The database is
// the source of truth: each pass reads the unfinished jobs from the store, so
// nothing is lost if a wake-up signal is missed or the process restarts.
type Worker struct {
	cfg  Config
	wake chan struct{}
	// mu serializes changes to jobs between the worker's goroutine and
	// Cancel, which runs in HTTP handlers.
	mu sync.Mutex
}

// New returns a Worker; call Run to start it.
func New(cfg Config) *Worker {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Worker{
		cfg: cfg,
		// Capacity 1: one pending signal is enough to trigger a new pass.
		wake: make(chan struct{}, 1),
	}
}

// Notify asks the worker to look at the jobs now instead of waiting for the
// next interval. It never blocks.
func (w *Worker) Notify() {
	select {
	case w.wake <- struct{}{}:
	default: // a pass is already pending
	}
}

// Run processes jobs until ctx is cancelled. Jobs left unfinished by a
// previous run are recovered on the first pass.
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

// pass advances every unfinished job by at most one step, oldest first.
func (w *Worker) pass(ctx context.Context) {
	jobs, err := w.cfg.Store.ListUnfinished(ctx)
	if err != nil {
		if ctx.Err() == nil {
			w.cfg.Logger.Error("list unfinished jobs", "error", err)
		}
		return
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			return
		}
		w.advance(ctx, j.ID)
	}
}

func (w *Worker) advance(ctx context.Context, id job.ID) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Reload under the lock: the job may have been cancelled meanwhile.
	j, err := w.cfg.Store.GetJob(ctx, id)
	if err != nil {
		w.cfg.Logger.Error("load job", "job_id", id, "error", err)
		return
	}

	switch j.Status {
	case job.StatusQueued:
		w.submit(ctx, j)
	case job.StatusProcessing:
		// Submission happens entirely under the lock, so a job seen in
		// processing here was left by a run that stopped mid-submission. The
		// provider may or may not have received it; failing is safer than
		// risking a duplicate print.
		w.fail(ctx, j, ReasonInterrupted)
	case job.StatusSubmitted, job.StatusPrinting:
		w.poll(ctx, j)
	}
}

func (w *Worker) submit(ctx context.Context, j *job.Job) {
	// Record processing before talking to the provider, so a crash during
	// submission is detectable on the next start.
	if err := j.StartProcessing(w.cfg.Now()); err != nil {
		w.cfg.Logger.Error("start processing", "job_id", j.ID, "error", err)
		return
	}
	if !w.save(ctx, j) {
		return
	}

	p, err := w.cfg.Printers.Get(ctx, j.Printer.ID)
	if err != nil || p.Status != printer.StatusOnline {
		w.fail(ctx, j, ReasonPrinterUnavailable)
		return
	}

	doc, err := w.cfg.Documents.Open(string(j.ID))
	if err != nil {
		w.cfg.Logger.Error("open document", "job_id", j.ID, "error", err)
		w.fail(ctx, j, ReasonDocumentMissing)
		return
	}
	defer doc.Close()

	providerJobID, err := w.cfg.Provider.Submit(ctx, p.Destination, doc, j.Options)
	switch {
	case ctx.Err() != nil:
		// Shutting down: leave the job in processing; the next start fails it
		// as interrupted.
		return
	case errors.Is(err, printing.ErrPrinterUnavailable):
		w.fail(ctx, j, ReasonPrinterUnavailable)
		return
	case err != nil:
		w.cfg.Logger.Error("submit job", "job_id", j.ID, "error", err)
		w.fail(ctx, j, ReasonSubmitFailed)
		return
	}

	if err := j.MarkSubmitted(providerJobID, w.cfg.Now()); err != nil {
		w.cfg.Logger.Error("mark submitted", "job_id", j.ID, "error", err)
		return
	}
	if w.save(ctx, j) {
		w.cfg.Logger.Info("job submitted", "job_id", j.ID, "provider_job_id", providerJobID)
	}
}

func (w *Worker) poll(ctx context.Context, j *job.Job) {
	st, err := w.cfg.Provider.JobStatus(ctx, j.ProviderJobID)
	if errors.Is(err, printing.ErrUnknownJob) {
		w.fail(ctx, j, ReasonProviderLostJob)
		return
	}
	if err != nil {
		// Possibly transient (provider unreachable): try again next pass.
		w.cfg.Logger.Warn("job status", "job_id", j.ID, "error", err)
		return
	}

	now := w.cfg.Now()
	switch st.State {
	case printing.JobPending:
		return
	case printing.JobProcessing:
		if j.Status == job.StatusSubmitted {
			if err := j.MarkPrinting(now); err == nil {
				w.save(ctx, j)
			}
		}
	case printing.JobCompleted:
		if err := j.Complete(now); err == nil {
			w.finish(ctx, j)
		}
	case printing.JobFailed:
		w.cfg.Logger.Warn("job failed at the provider", "job_id", j.ID, "message", st.Message)
		w.fail(ctx, j, ReasonPrinterError)
	case printing.JobCancelled:
		// Cancelled outside PrintHub, e.g. on the CUPS web interface.
		if err := j.Cancel(now); err == nil {
			w.finish(ctx, j)
		}
	}
}

// ErrNotCancellable is matched (with errors.Is) when a job can no longer be
// cancelled. It wraps job.ErrInvalidTransition.
var ErrNotCancellable = fmt.Errorf("%w: job cannot be cancelled", job.ErrInvalidTransition)

// Cancel cancels a job at the provider (if it was submitted) and in PrintHub.
func (w *Worker) Cancel(ctx context.Context, id job.ID) (*job.Job, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	j, err := w.cfg.Store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if !job.CanTransition(j.Status, job.StatusCancelled) {
		return nil, ErrNotCancellable
	}

	if j.Status == job.StatusSubmitted || j.Status == job.StatusPrinting {
		err := w.cfg.Provider.Cancel(ctx, j.ProviderJobID)
		switch {
		case errors.Is(err, printing.ErrJobFinished):
			// Too late: the next pass records how it ended.
			return nil, ErrNotCancellable
		case err != nil && !errors.Is(err, printing.ErrUnknownJob):
			return nil, fmt.Errorf("cancel at provider: %w", err)
		}
	}

	if err := j.Cancel(w.cfg.Now()); err != nil {
		return nil, err
	}
	if err := w.cfg.Store.SaveJob(ctx, j); err != nil {
		return nil, err
	}
	w.removeDocument(j.ID)
	w.cfg.Logger.Info("job cancelled", "job_id", j.ID)
	return j, nil
}

// CleanDocuments removes stored documents whose job is missing or finished:
// leftovers of a crash between storing an upload and saving its job. Call it
// before accepting uploads, so it never races with one in progress.
func (w *Worker) CleanDocuments(ctx context.Context) error {
	ids, err := w.cfg.Documents.IDs()
	if err != nil {
		return err
	}
	for _, id := range ids {
		j, err := w.cfg.Store.GetJob(ctx, job.ID(id))
		switch {
		case errors.Is(err, job.ErrNotFound), err == nil && j.Status.Terminal():
			w.removeDocument(job.ID(id))
			w.cfg.Logger.Info("removed orphan document", "job_id", id)
		case err != nil:
			return err
		}
	}
	return nil
}

func (w *Worker) fail(ctx context.Context, j *job.Job, reason string) {
	if err := j.Fail(reason, w.cfg.Now()); err != nil {
		w.cfg.Logger.Error("fail job", "job_id", j.ID, "error", err)
		return
	}
	w.finish(ctx, j)
}

// finish saves a job that reached a terminal status and removes its
// document, which is no longer needed.
func (w *Worker) finish(ctx context.Context, j *job.Job) {
	if !w.save(ctx, j) {
		return
	}
	w.removeDocument(j.ID)
	w.cfg.Logger.Info("job finished", "job_id", j.ID, "status", j.Status, "reason", j.FailureReason)
}

func (w *Worker) save(ctx context.Context, j *job.Job) bool {
	if err := w.cfg.Store.SaveJob(ctx, j); err != nil {
		w.cfg.Logger.Error("save job", "job_id", j.ID, "error", err)
		return false
	}
	return true
}

func (w *Worker) removeDocument(id job.ID) {
	if err := w.cfg.Documents.Remove(string(id)); err != nil {
		w.cfg.Logger.Error("remove document", "job_id", id, "error", err)
	}
}
