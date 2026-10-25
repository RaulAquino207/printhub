// Package printing holds the vocabulary shared by printing providers (mock,
// CUPS) and their consumers: the state of a job as a provider reports it.
package printing

import "errors"

// JobState is a job's state as reported by a provider.
type JobState string

const (
	JobPending    JobState = "pending"    // accepted, not started yet
	JobProcessing JobState = "processing" // being printed
	JobCompleted  JobState = "completed"  // finished; does not prove paper came out
	JobFailed     JobState = "failed"
	JobCancelled  JobState = "cancelled"
)

// JobStatus is a provider's report about one job.
type JobStatus struct {
	State JobState
	// Message explains a failure, when the provider gives one.
	Message string
}

var (
	// ErrUnknownJob means the provider has no job with that ID, e.g. it was
	// restarted and lost its state.
	ErrUnknownJob = errors.New("printing: unknown job")
	// ErrJobFinished means the job can no longer be cancelled.
	ErrJobFinished = errors.New("printing: job already finished")
	// ErrPrinterUnavailable means the destination cannot accept jobs now.
	ErrPrinterUnavailable = errors.New("printing: printer unavailable")
)
