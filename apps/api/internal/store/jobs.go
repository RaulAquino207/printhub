package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/RaulAquino207/printhub/apps/api/internal/job"
)

// SaveJob inserts a job or updates the fields that change over its lifecycle
// (status, provider job ID, failure reason, update time) and appends the
// transitions not yet stored. The printer snapshot, document and options are
// written once and never changed.
func (s *Store) SaveJob(ctx context.Context, j *job.Job) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save job %s: %w", j.ID, err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO jobs (
			id, printer_id, printer_name, printer_model, printer_simulated,
			document_name, document_size_bytes,
			copies, color_mode, paper_size, orientation, duplex, page_ranges,
			status, provider_job_id, failure_reason, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			status          = excluded.status,
			provider_job_id = excluded.provider_job_id,
			failure_reason  = excluded.failure_reason,
			updated_at      = excluded.updated_at`,
		j.ID, j.Printer.ID, j.Printer.Name, j.Printer.Model, j.Printer.Simulated,
		j.Document.Name, j.Document.SizeBytes,
		j.Options.Copies, j.Options.ColorMode, j.Options.PaperSize, j.Options.Orientation, j.Options.Duplex, j.Options.PageRanges,
		j.Status, j.ProviderJobID, j.FailureReason, formatTime(j.CreatedAt()), formatTime(j.UpdatedAt()),
	)
	if err != nil {
		return fmt.Errorf("save job %s: %w", j.ID, err)
	}

	// History only grows, so rows already stored are skipped.
	for seq, tr := range j.History {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO job_transitions (job_id, seq, status, at) VALUES (?, ?, ?, ?)
			 ON CONFLICT (job_id, seq) DO NOTHING`,
			j.ID, seq, tr.Status, formatTime(tr.At),
		)
		if err != nil {
			return fmt.Errorf("save job %s: transition %d: %w", j.ID, seq, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save job %s: commit: %w", j.ID, err)
	}
	return nil
}

const jobColumns = `
	id, printer_id, printer_name, printer_model, printer_simulated,
	document_name, document_size_bytes,
	copies, color_mode, paper_size, orientation, duplex, page_ranges,
	status, provider_job_id, failure_reason`

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*job.Job, error) {
	var j job.Job
	err := row.Scan(
		&j.ID, &j.Printer.ID, &j.Printer.Name, &j.Printer.Model, &j.Printer.Simulated,
		&j.Document.Name, &j.Document.SizeBytes,
		&j.Options.Copies, &j.Options.ColorMode, &j.Options.PaperSize, &j.Options.Orientation, &j.Options.Duplex, &j.Options.PageRanges,
		&j.Status, &j.ProviderJobID, &j.FailureReason,
	)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// GetJob returns the job with the given ID, or job.ErrNotFound.
func (s *Store) GetJob(ctx context.Context, id job.ID) (*job.Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, job.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job %s: %w", id, err)
	}

	if err := s.loadHistory(ctx, []*job.Job{j}); err != nil {
		return nil, fmt.Errorf("get job %s: %w", id, err)
	}
	return j, nil
}

// ListUnfinished returns the jobs that were not in a terminal status, oldest
// first. At startup they are what the worker must recover.
func (s *Store) ListUnfinished(ctx context.Context) ([]*job.Job, error) {
	statuses := job.UnfinishedStatuses()
	args := make([]any, len(statuses))
	for i, st := range statuses {
		args[i] = st
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+jobColumns+` FROM jobs
		 WHERE status IN (`+placeholders(len(args))+`)
		 ORDER BY created_at, id`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list unfinished jobs: %w", err)
	}

	var jobs []*job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("list unfinished jobs: %w", err)
		}
		jobs = append(jobs, j)
	}
	// Close (and check iteration errors) before the next query: with a single
	// connection, open rows would block loadHistory forever.
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("list unfinished jobs: %w", err)
	}

	if err := s.loadHistory(ctx, jobs); err != nil {
		return nil, fmt.Errorf("list unfinished jobs: %w", err)
	}
	return jobs, nil
}

// loadHistory fills the History of each job with a single query.
func (s *Store) loadHistory(ctx context.Context, jobs []*job.Job) error {
	if len(jobs) == 0 {
		return nil
	}

	byID := make(map[job.ID]*job.Job, len(jobs))
	args := make([]any, len(jobs))
	for i, j := range jobs {
		byID[j.ID] = j
		args[i] = j.ID
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT job_id, status, at FROM job_transitions
		 WHERE job_id IN (`+placeholders(len(args))+`)
		 ORDER BY job_id, seq`,
		args...,
	)
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id job.ID
			tr job.Transition
			at string
		)
		if err := rows.Scan(&id, &tr.Status, &at); err != nil {
			return fmt.Errorf("load history: %w", err)
		}
		if tr.At, err = parseTime(at); err != nil {
			return fmt.Errorf("load history of %s: %w", id, err)
		}
		byID[id].History = append(byID[id].History, tr)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	for _, j := range jobs {
		if len(j.History) == 0 {
			return fmt.Errorf("load history: job %s has no transitions", j.ID)
		}
	}
	return nil
}

// placeholders returns "?, ?, ?" for n arguments.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
