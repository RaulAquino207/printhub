package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"

	"github.com/RaulAquino207/printhub/apps/api/internal/document"
	"github.com/RaulAquino207/printhub/apps/api/internal/job"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// JobStore persists jobs. *store.Store satisfies it.
type JobStore interface {
	SaveJob(ctx context.Context, j *job.Job) error
	GetJob(ctx context.Context, id job.ID) (*job.Job, error)
	ListJobs(ctx context.Context, statuses []job.Status, limit int) ([]*job.Job, error)
}

// JobRunner is the background worker as seen by HTTP handlers.
// *worker.Worker satisfies it.
type JobRunner interface {
	// Notify wakes the worker after a job is queued.
	Notify()
	// Cancel cancels a job; the error matches job.ErrInvalidTransition when
	// the job can no longer be cancelled.
	Cancel(ctx context.Context, id job.ID) (*job.Job, error)
}

// DocumentStore keeps uploaded files. *document.Storage satisfies it.
type DocumentStore interface {
	Save(id string, r io.Reader) (int64, error)
	Remove(id string) error
	MaxBytes() int64
}

const (
	// maxFieldBytes bounds the non-file form fields (printerId, options).
	maxFieldBytes = 64 << 10
	// multipartOverhead is allowed on top of the file for boundaries, part
	// headers and the other fields.
	multipartOverhead = 1 << 20
	maxFilenameRunes  = 200
	defaultFilename   = "documento.pdf"

	defaultListLimit = 50
	maxListLimit     = 200
)

var allStatuses = []job.Status{
	job.StatusQueued, job.StatusProcessing, job.StatusSubmitted, job.StatusPrinting,
	job.StatusCompleted, job.StatusFailed, job.StatusCancelled,
}

// optionsRequest is the JSON contract of the "options" form field.
type optionsRequest struct {
	Copies      int                 `json:"copies"`
	ColorMode   printer.ColorMode   `json:"colorMode"`
	PaperSize   printer.PaperSize   `json:"paperSize"`
	Orientation printer.Orientation `json:"orientation"`
	Duplex      bool                `json:"duplex"`
	PageRanges  string              `json:"pageRanges"`
}

// createJob accepts multipart/form-data with the fields "printerId",
// "options" (JSON) and "file" (the PDF), in any order.
//
// The body is read as a stream: the file goes straight to disk, never fully
// into memory. If anything fails after the file is stored, it is removed.
func (s *server) createJob(w http.ResponseWriter, r *http.Request) {
	// Hard cap on the whole body, whatever the client declares.
	r.Body = http.MaxBytesReader(w, r.Body, s.documents.MaxBytes()+multipartOverhead)

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expected a multipart/form-data body")
		return
	}

	id := job.ID(rand.Text())
	var (
		printerID   string
		optionsJSON []byte
		doc         job.Document
		fileStored  bool
		fileErr     error
		jobSaved    bool
	)
	// Whatever happens next, a stored file without a saved job is removed.
	defer func() {
		if fileStored && !jobSaved {
			if err := s.documents.Remove(string(id)); err != nil {
				s.logger.Error("remove orphan document", "job_id", id, "error", err)
			}
		}
	}()

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			s.uploadError(w, err)
			return
		}

		switch part.FormName() {
		case "printerId":
			var b []byte
			b, err = readField(part)
			printerID = strings.TrimSpace(string(b))
		case "options":
			optionsJSON, err = readField(part)
		case "file":
			if fileStored || fileErr != nil {
				err = errors.New("more than one file")
				break
			}
			var size int64
			size, fileErr = s.documents.Save(string(id), part)
			if fileErr == nil {
				fileStored = true
				doc = job.Document{Name: cleanFilename(part.FileName()), SizeBytes: size}
			}
		}
		part.Close()

		if err != nil {
			s.uploadError(w, err)
			return
		}
		// Content problems are reported with the other fields below; size and
		// I/O problems end the request now.
		if fileErr != nil && !errors.Is(fileErr, document.ErrNotPDF) {
			s.uploadError(w, fileErr)
			return
		}
	}

	var fields []fieldError
	add := func(field, reason string) { fields = append(fields, fieldError{Field: field, Reason: reason}) }

	switch {
	case errors.Is(fileErr, document.ErrNotPDF):
		add("file", "not_pdf")
	case !fileStored:
		add("file", "required")
	}

	var p printer.Printer
	if printerID == "" {
		add("printerId", "required")
	} else {
		p, err = s.printers.Get(r.Context(), printer.ID(printerID))
		switch {
		case errors.Is(err, printer.ErrNotFound):
			add("printerId", "not_found")
		case err != nil:
			s.logger.Error("get printer", "id", printerID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not create the job")
			return
		case p.Status != printer.StatusOnline:
			add("printerId", "unavailable")
		}
	}

	var opts printer.Options
	if optionsJSON == nil {
		add("options", "required")
	} else if opts, err = decodeOptions(optionsJSON); err != nil {
		add("options", "invalid_format")
	} else if p.ID != "" && p.Status == printer.StatusOnline {
		var optsErr *printer.OptionsError
		if errors.As(opts.Validate(p.Capabilities), &optsErr) {
			for _, f := range optsErr.Fields {
				add(f.Field, f.Reason)
			}
		}
	}

	if len(fields) > 0 {
		writeValidationError(w, fields)
		return
	}

	j := job.New(id, job.SnapshotOf(p), doc, opts, time.Now())
	if user, ok := userFrom(r.Context()); ok {
		j.CreatedBy = user.Username
	}
	if err := s.jobs.SaveJob(r.Context(), j); err != nil {
		s.logger.Error("save job", "job_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create the job")
		return
	}
	jobSaved = true
	s.worker.Notify()

	s.logger.Info("job created", "job_id", id, "printer_id", p.ID, "size_bytes", doc.SizeBytes)
	w.Header().Set("Location", "/api/v1/jobs/"+string(id))
	writeJSON(w, http.StatusCreated, toJobResponse(j))
}

// uploadError answers for failures while reading the body.
func (s *server) uploadError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr), errors.Is(err, document.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			fmt.Sprintf("the file exceeds %d bytes", s.documents.MaxBytes()))
	case errors.Is(err, errFieldTooLarge):
		writeError(w, http.StatusBadRequest, "invalid_request", "a form field is too large")
	default:
		// Malformed multipart bodies end up here; the details stay in the log.
		s.logger.Warn("read upload", "error", err)
		writeError(w, http.StatusBadRequest, "invalid_request", "could not read the upload")
	}
}

var errFieldTooLarge = errors.New("form field too large")

// readField reads a small form field, refusing anything above maxFieldBytes.
func readField(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxFieldBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFieldBytes {
		return nil, errFieldTooLarge
	}
	return b, nil
}

func decodeOptions(data []byte) (printer.Options, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req optionsRequest
	if err := dec.Decode(&req); err != nil {
		return printer.Options{}, err
	}
	return printer.Options(req), nil
}

// cleanFilename keeps a display name for the document. It is only metadata:
// files are stored under the job ID, never under this name.
func cleanFilename(name string) string {
	// Some clients send a full path, with either separator.
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if runes := []rune(name); len(runes) > maxFilenameRunes {
		name = string(runes[:maxFilenameRunes])
	}
	if name == "" || name == "." || name == "/" {
		return defaultFilename
	}
	return name
}

type jobListResponse struct {
	Jobs []jobResponse `json:"jobs"`
}

// listJobs answers GET /api/v1/jobs?status=queued,printing&limit=50, newest
// first. Without status, every status is included.
func (s *server) listJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var fields []fieldError

	var statuses []job.Status
	if raw := q.Get("status"); raw != "" {
		for part := range strings.SplitSeq(raw, ",") {
			st := job.Status(strings.TrimSpace(part))
			if !slices.Contains(allStatuses, st) {
				fields = append(fields, fieldError{Field: "status", Reason: "invalid"})
				break
			}
			statuses = append(statuses, st)
		}
	}

	limit := defaultListLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxListLimit {
			fields = append(fields, fieldError{Field: "limit", Reason: "out_of_range"})
		}
		limit = n
	}

	if len(fields) > 0 {
		writeValidationError(w, fields)
		return
	}

	jobs, err := s.jobs.ListJobs(r.Context(), statuses, limit)
	if err != nil {
		s.logger.Error("list jobs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not list jobs")
		return
	}

	resp := jobListResponse{Jobs: make([]jobResponse, 0, len(jobs))}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, toJobResponse(j))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) getJob(w http.ResponseWriter, r *http.Request) {
	id := job.ID(chi.URLParam(r, "id"))

	j, err := s.jobs.GetJob(r.Context(), id)
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if err != nil {
		s.logger.Error("get job", "job_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not get the job")
		return
	}
	writeJSON(w, http.StatusOK, toJobResponse(j))
}

func (s *server) cancelJob(w http.ResponseWriter, r *http.Request) {
	id := job.ID(chi.URLParam(r, "id"))

	j, err := s.worker.Cancel(r.Context(), id)
	switch {
	case errors.Is(err, job.ErrNotFound):
		writeError(w, http.StatusNotFound, "job_not_found", "job not found")
	case errors.Is(err, job.ErrInvalidTransition):
		writeError(w, http.StatusConflict, "job_not_cancellable", "the job can no longer be cancelled")
	case err != nil:
		s.logger.Error("cancel job", "job_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not cancel the job")
	default:
		writeJSON(w, http.StatusOK, toJobResponse(j))
	}
}

type jobResponse struct {
	ID            string               `json:"id"`
	Status        job.Status           `json:"status"`
	Printer       jobPrinterResponse   `json:"printer"`
	Document      documentResponse     `json:"document"`
	Options       optionsRequest       `json:"options"`
	ProviderJobID string               `json:"providerJobId,omitempty"`
	FailureReason string               `json:"failureReason,omitempty"`
	CreatedBy     string               `json:"createdBy"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
	History       []transitionResponse `json:"history"`
}

type jobPrinterResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	Simulated bool   `json:"simulated"`
}

type documentResponse struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
}

type transitionResponse struct {
	Status job.Status `json:"status"`
	At     time.Time  `json:"at"`
}

// toJobResponse maps a job to its JSON contract. Times are always UTC, so
// clients see the same representation whether the job came from memory or
// from the database.
func toJobResponse(j *job.Job) jobResponse {
	history := make([]transitionResponse, len(j.History))
	for i, tr := range j.History {
		history[i] = transitionResponse{Status: tr.Status, At: tr.At.UTC()}
	}
	return jobResponse{
		ID:     string(j.ID),
		Status: j.Status,
		Printer: jobPrinterResponse{
			ID:        string(j.Printer.ID),
			Name:      j.Printer.Name,
			Model:     j.Printer.Model,
			Simulated: j.Printer.Simulated,
		},
		Document:      documentResponse{Name: j.Document.Name, SizeBytes: j.Document.SizeBytes},
		Options:       optionsRequest(j.Options),
		ProviderJobID: j.ProviderJobID,
		FailureReason: j.FailureReason,
		CreatedBy:     j.CreatedBy,
		CreatedAt:     j.CreatedAt().UTC(),
		UpdatedAt:     j.UpdatedAt().UTC(),
		History:       history,
	}
}
