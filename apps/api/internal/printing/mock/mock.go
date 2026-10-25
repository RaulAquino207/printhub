// Package mock is a simulated printing provider for development and tests.
// Nothing it reports comes from real hardware.
package mock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing"
)

// ErrUnknownDestination is returned for a destination the mock does not know.
var ErrUnknownDestination = errors.New("mock: unknown destination")

// Printer is the simulated state behind one destination.
type Printer struct {
	Destination  string
	Status       printer.Status
	Capabilities printer.Capabilities
	// FailJobs makes every job on this printer fail once printing starts,
	// to exercise failure handling.
	FailJobs bool
}

// Timing controls how long simulated jobs take.
type Timing struct {
	// Queue is the time a job stays pending after it is submitted.
	Queue time.Duration
	// Print is the time a job spends processing before it finishes.
	Print time.Duration
}

// simJob is a submitted job. Its state is derived from the clock, so the mock
// needs no goroutines of its own.
type simJob struct {
	destination string
	submittedAt time.Time
	cancelled   bool
}

// Provider serves the state of simulated printers and simulates jobs. It is
// safe for concurrent use: the worker and HTTP handlers call it at the same time.
type Provider struct {
	printers map[string]Printer
	timing   Timing
	now      func() time.Time

	mu     sync.Mutex // guards jobs and nextID
	jobs   map[string]*simJob
	nextID int
}

// New returns a Provider that simulates the given printers. Jobs complete
// immediately unless Timing is set with SetTiming.
func New(printers ...Printer) *Provider {
	byDestination := make(map[string]Printer, len(printers))
	for _, p := range printers {
		byDestination[p.Destination] = p
	}
	return &Provider{
		printers: byDestination,
		now:      time.Now,
		jobs:     make(map[string]*simJob),
	}
}

// SetTiming sets how long simulated jobs take.
func (p *Provider) SetTiming(t Timing) { p.timing = t }

// SetClock replaces the clock, so tests can move time forward.
func (p *Provider) SetClock(now func() time.Time) { p.now = now }

// Status returns the simulated status of the destination.
func (p *Provider) Status(_ context.Context, destination string) (printer.Status, error) {
	sp, ok := p.printers[destination]
	if !ok {
		return "", ErrUnknownDestination
	}
	return sp.Status, nil
}

// Capabilities returns the simulated capabilities of the destination.
func (p *Provider) Capabilities(_ context.Context, destination string) (printer.Capabilities, error) {
	sp, ok := p.printers[destination]
	if !ok {
		return printer.Capabilities{}, ErrUnknownDestination
	}
	return sp.Capabilities, nil
}

// Submit reads the whole document, as a real provider would, and queues a
// simulated job.
func (p *Provider) Submit(ctx context.Context, destination string, doc io.Reader, _ printer.Options) (string, error) {
	sp, ok := p.printers[destination]
	if !ok {
		return "", ErrUnknownDestination
	}
	if sp.Status != printer.StatusOnline {
		return "", printing.ErrPrinterUnavailable
	}
	if _, err := io.Copy(io.Discard, doc); err != nil {
		return "", fmt.Errorf("mock: read document: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextID++
	id := fmt.Sprintf("mock-%d", p.nextID)
	p.jobs[id] = &simJob{destination: destination, submittedAt: p.now()}
	return id, nil
}

// JobStatus derives the job's state from the time elapsed since submission.
func (p *Provider) JobStatus(_ context.Context, id string) (printing.JobStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	j, ok := p.jobs[id]
	if !ok {
		return printing.JobStatus{}, printing.ErrUnknownJob
	}
	return p.stateOf(j), nil
}

// Cancel stops a pending or processing job.
func (p *Provider) Cancel(_ context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	j, ok := p.jobs[id]
	if !ok {
		return printing.ErrUnknownJob
	}
	switch p.stateOf(j).State {
	case printing.JobPending, printing.JobProcessing:
		j.cancelled = true
		return nil
	default:
		return printing.ErrJobFinished
	}
}

// stateOf must be called with p.mu held.
func (p *Provider) stateOf(j *simJob) printing.JobStatus {
	if j.cancelled {
		return printing.JobStatus{State: printing.JobCancelled}
	}
	elapsed := p.now().Sub(j.submittedAt)
	switch {
	case elapsed < p.timing.Queue:
		return printing.JobStatus{State: printing.JobPending}
	case p.printers[j.destination].FailJobs:
		return printing.JobStatus{State: printing.JobFailed, Message: "simulated paper jam"}
	case elapsed < p.timing.Queue+p.timing.Print:
		return printing.JobStatus{State: printing.JobProcessing}
	default:
		return printing.JobStatus{State: printing.JobCompleted}
	}
}
