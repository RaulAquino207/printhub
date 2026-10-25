// Package job models print jobs and the rules for moving them through their
// lifecycle.
package job

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// ID identifies a print job.
type ID string

// Status is a stage of the job lifecycle:
//
//	queued → processing → submitted → printing → completed
//	any non-terminal status → failed
//	queued, submitted, printing → cancelled
type Status string

const (
	// StatusQueued: accepted by PrintHub, waiting for the worker.
	StatusQueued Status = "queued"
	// StatusProcessing: the worker is validating the document and sending it to the provider.
	StatusProcessing Status = "processing"
	// StatusSubmitted: the provider accepted the job and returned its own job ID.
	StatusSubmitted Status = "submitted"
	// StatusPrinting: the provider reports the job as being processed.
	StatusPrinting Status = "printing"
	// StatusCompleted: the provider reports the job as finished. This does not
	// prove that paper came out of the printer.
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// transitions lists, for each status, the statuses it may move to. Terminal
// statuses have no entry.
//
// processing cannot be cancelled: the document is being handed to the provider
// at that moment, so the outcome is either submitted or failed.
// submitted may jump straight to completed, because a short job can finish
// before PrintHub ever observes it printing.
var transitions = map[Status][]Status{
	StatusQueued:     {StatusProcessing, StatusFailed, StatusCancelled},
	StatusProcessing: {StatusSubmitted, StatusFailed},
	StatusSubmitted:  {StatusPrinting, StatusCompleted, StatusFailed, StatusCancelled},
	StatusPrinting:   {StatusCompleted, StatusFailed, StatusCancelled},
}

// UnfinishedStatuses lists the non-terminal statuses in lifecycle order: the
// jobs in these statuses still need work after a restart.
func UnfinishedStatuses() []Status {
	return []Status{StatusQueued, StatusProcessing, StatusSubmitted, StatusPrinting}
}

// Terminal reports whether no further transition is possible.
func (s Status) Terminal() bool {
	_, ok := transitions[s]
	return !ok
}

// ErrNotFound is returned when no job has the given ID.
var ErrNotFound = errors.New("job not found")

// ErrInvalidTransition is matched by every *TransitionError, so callers can
// write errors.Is(err, job.ErrInvalidTransition).
var ErrInvalidTransition = errors.New("invalid job status transition")

// TransitionError describes a rejected status change. Use errors.As to read
// From and To.
type TransitionError struct {
	From, To Status
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("job: cannot move from %s to %s", e.From, e.To)
}

// Unwrap makes errors.Is(err, ErrInvalidTransition) succeed.
func (e *TransitionError) Unwrap() error {
	return ErrInvalidTransition
}

// PrinterSnapshot is the printer as it was when the job was created. Jobs keep
// it so their history stays meaningful even if the printer is later renamed,
// reconfigured or removed.
type PrinterSnapshot struct {
	ID        printer.ID
	Name      string
	Model     string
	Simulated bool
}

// SnapshotOf captures the fields of p that a job keeps.
func SnapshotOf(p printer.Printer) PrinterSnapshot {
	return PrinterSnapshot{ID: p.ID, Name: p.Name, Model: p.Model, Simulated: p.Simulated}
}

// Document is the metadata of the uploaded file. The file itself is stored
// elsewhere.
type Document struct {
	Name      string
	SizeBytes int64
}

// Transition records when a job entered a status.
type Transition struct {
	Status Status
	At     time.Time
}

// Job is a print job and its history.
type Job struct {
	ID       ID
	Printer  PrinterSnapshot
	Document Document
	Options  printer.Options
	Status   Status
	// ProviderJobID is the job ID assigned by the printing provider (e.g. the
	// CUPS job ID), known from StatusSubmitted on.
	ProviderJobID string
	// FailureReason explains why the job failed; empty otherwise.
	FailureReason string
	// CreatedBy is the username of whoever sent the job.
	CreatedBy string
	// History holds every status the job entered, oldest first. The first
	// entry is the creation time.
	History []Transition
}

// New returns a queued job. now is passed in instead of read from the clock,
// which keeps the lifecycle deterministic and easy to test.
func New(id ID, p PrinterSnapshot, doc Document, opts printer.Options, now time.Time) *Job {
	return &Job{
		ID:       id,
		Printer:  p,
		Document: doc,
		Options:  opts,
		Status:   StatusQueued,
		History:  []Transition{{Status: StatusQueued, At: now}},
	}
}

// CreatedAt is when the job was queued.
func (j *Job) CreatedAt() time.Time {
	return j.History[0].At
}

// UpdatedAt is when the job last changed status.
func (j *Job) UpdatedAt() time.Time {
	return j.History[len(j.History)-1].At
}

// StartProcessing moves a queued job to processing.
func (j *Job) StartProcessing(now time.Time) error {
	return j.moveTo(StatusProcessing, now)
}

// MarkSubmitted records that the provider accepted the job.
func (j *Job) MarkSubmitted(providerJobID string, now time.Time) error {
	if err := j.moveTo(StatusSubmitted, now); err != nil {
		return err
	}
	j.ProviderJobID = providerJobID
	return nil
}

// MarkPrinting records that the provider is processing the job.
func (j *Job) MarkPrinting(now time.Time) error {
	return j.moveTo(StatusPrinting, now)
}

// Complete records that the provider finished the job.
func (j *Job) Complete(now time.Time) error {
	return j.moveTo(StatusCompleted, now)
}

// Fail moves the job to failed with a human-readable reason.
func (j *Job) Fail(reason string, now time.Time) error {
	if err := j.moveTo(StatusFailed, now); err != nil {
		return err
	}
	j.FailureReason = reason
	return nil
}

// Cancel moves the job to cancelled.
func (j *Job) Cancel(now time.Time) error {
	return j.moveTo(StatusCancelled, now)
}

// moveTo is the single place where status changes happen.
func (j *Job) moveTo(to Status, now time.Time) error {
	if !CanTransition(j.Status, to) {
		return &TransitionError{From: j.Status, To: to}
	}
	j.Status = to
	j.History = append(j.History, Transition{Status: to, At: now})
	return nil
}

// CanTransition reports whether a job may move from one status to another.
func CanTransition(from, to Status) bool {
	return slices.Contains(transitions[from], to)
}
