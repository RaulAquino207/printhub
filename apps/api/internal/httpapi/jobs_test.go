package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/document"
	"github.com/RaulAquino207/printhub/apps/api/internal/job"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// fakeJobStore keeps jobs in memory.
type fakeJobStore struct {
	mu        sync.Mutex
	jobs      map[job.ID]*job.Job
	saveErr   error
	lastLimit int
}

func (f *fakeJobStore) SaveJob(_ context.Context, j *job.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.jobs[j.ID] = j
	return nil
}

// ListJobs returns the stored jobs with a matching status, ignoring order.
func (f *fakeJobStore) ListJobs(_ context.Context, statuses []job.Status, limit int) ([]*job.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLimit = limit
	var out []*job.Job
	for _, j := range f.jobs {
		if len(statuses) == 0 || slices.Contains(statuses, j.Status) {
			out = append(out, j)
		}
	}
	return out, nil
}

func (f *fakeJobStore) GetJob(_ context.Context, id job.ID) (*job.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return nil, job.ErrNotFound
	}
	return j, nil
}

// fakeWorker records notifications and cancels by delegating to a function.
type fakeWorker struct {
	notified int
	cancel   func(job.ID) (*job.Job, error)
}

func (f *fakeWorker) Notify() { f.notified++ }

func (f *fakeWorker) Cancel(_ context.Context, id job.ID) (*job.Job, error) {
	return f.cancel(id)
}

const testMaxBytes = 1024

var uploadPrinters = []printer.Printer{
	{
		Config: printer.Config{ID: "mono", Name: "Mono", Model: "M", Simulated: true},
		Status: printer.StatusOnline,
		Capabilities: printer.Capabilities{
			ColorModes:   []printer.ColorMode{printer.ColorModeMonochrome},
			PaperSizes:   []printer.PaperSize{"A4"},
			Orientations: []printer.Orientation{printer.OrientationPortrait},
			PageRanges:   true,
			MaxCopies:    5,
		},
	},
	{Config: printer.Config{ID: "off", Name: "Off"}, Status: printer.StatusOffline},
}

type uploadEnv struct {
	router  http.Handler
	jobs    *fakeJobStore
	worker  *fakeWorker
	docsDir string
}

func newUploadEnv(t *testing.T) *uploadEnv {
	t.Helper()
	docsDir := filepath.Join(t.TempDir(), "documents")
	docs, err := document.NewStorage(docsDir, testMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	jobs := &fakeJobStore{jobs: map[job.ID]*job.Job{}}
	worker := &fakeWorker{}
	router := NewRouter(Dependencies{
		Logger:    slog.New(slog.DiscardHandler),
		Printers:  fakeDirectory{printers: uploadPrinters},
		Jobs:      jobs,
		Documents: docs,
		Worker:    worker,
	})
	return &uploadEnv{router: router, jobs: jobs, worker: worker, docsDir: docsDir}
}

func (e *uploadEnv) storedFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(e.docsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// form describes a multipart body; nil fields are omitted.
type form struct {
	printerID *string
	options   *string
	file      []byte
	filename  string
}

func ptr(s string) *string { return &s }

const validOptions = `{"copies":2,"colorMode":"monochrome","paperSize":"A4","orientation":"portrait","duplex":false,"pageRanges":"1-2"}`

var validPDF = []byte("%PDF-1.7\n% test document\n%%EOF\n")

func validForm() form {
	return form{printerID: ptr("mono"), options: ptr(validOptions), file: validPDF, filename: "Relatório final.pdf"}
}

func (e *uploadEnv) post(t *testing.T, f form) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if f.printerID != nil {
		mw.WriteField("printerId", *f.printerID)
	}
	if f.options != nil {
		mw.WriteField("options", *f.options)
	}
	if f.file != nil {
		fw, err := mw.CreateFormFile("file", f.filename)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(f.file)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body.Error
}

func TestCreateJob(t *testing.T) {
	env := newUploadEnv(t)

	rec := env.post(t, validForm())

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var resp jobResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != job.StatusQueued || resp.Printer.ID != "mono" || !resp.Printer.Simulated {
		t.Errorf("job = %+v", resp)
	}
	if resp.Document.Name != "Relatório final.pdf" || resp.Document.SizeBytes != int64(len(validPDF)) {
		t.Errorf("document = %+v", resp.Document)
	}
	if resp.Options.Copies != 2 || resp.Options.PageRanges != "1-2" {
		t.Errorf("options = %+v", resp.Options)
	}
	if got := rec.Header().Get("Location"); got != "/api/v1/jobs/"+resp.ID {
		t.Errorf("Location = %q", got)
	}

	// The file is stored under the job ID, not the client's filename.
	if files := env.storedFiles(t); !slices.Equal(files, []string{resp.ID + ".pdf"}) {
		t.Errorf("stored files = %v, want [%s.pdf]", files, resp.ID)
	}
	if _, err := env.jobs.GetJob(context.Background(), job.ID(resp.ID)); err != nil {
		t.Errorf("job not saved: %v", err)
	}
	if env.worker.notified != 1 {
		t.Errorf("worker notified %d times, want 1", env.worker.notified)
	}

	// And it can be read back.
	get := httptest.NewRecorder()
	env.router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+resp.ID, nil))
	if get.Code != http.StatusOK {
		t.Errorf("GET status = %d, want 200", get.Code)
	}
}

func TestCreateJobFileFirst(t *testing.T) {
	env := newUploadEnv(t)

	// Parts in a different order: file before the other fields.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "a.pdf")
	fw.Write(validPDF)
	mw.WriteField("options", validOptions)
	mw.WriteField("printerId", "mono")
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
}

func TestCreateJobValidation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*form)
		want   []fieldError
	}{
		{
			name:   "not a pdf",
			modify: func(f *form) { f.file = []byte("<html></html>") },
			want:   []fieldError{{"file", "not_pdf"}},
		},
		{
			name: "missing everything",
			modify: func(f *form) {
				f.file, f.printerID, f.options = nil, nil, nil
			},
			want: []fieldError{{"file", "required"}, {"printerId", "required"}, {"options", "required"}},
		},
		{
			name:   "unknown printer",
			modify: func(f *form) { f.printerID = ptr("nope") },
			want:   []fieldError{{"printerId", "not_found"}},
		},
		{
			name:   "offline printer",
			modify: func(f *form) { f.printerID = ptr("off") },
			want:   []fieldError{{"printerId", "unavailable"}},
		},
		{
			name:   "malformed options",
			modify: func(f *form) { f.options = ptr(`{"copies":`) },
			want:   []fieldError{{"options", "invalid_format"}},
		},
		{
			name:   "unknown option field",
			modify: func(f *form) { f.options = ptr(`{"copies":1,"staple":true}`) },
			want:   []fieldError{{"options", "invalid_format"}},
		},
		{
			name: "options not supported by the printer",
			modify: func(f *form) {
				f.options = ptr(`{"copies":9,"colorMode":"color","paperSize":"A4","orientation":"portrait"}`)
			},
			want: []fieldError{{"copies", "out_of_range"}, {"colorMode", "unsupported"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newUploadEnv(t)
			f := validForm()
			tt.modify(&f)

			rec := env.post(t, f)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body)
			}
			body := decodeError(t, rec)
			if body.Code != "validation_failed" || !slices.Equal(body.Fields, tt.want) {
				t.Errorf("error = %+v, want fields %v", body, tt.want)
			}
			if files := env.storedFiles(t); len(files) != 0 {
				t.Errorf("files left behind: %v", files)
			}
			if len(env.jobs.jobs) != 0 {
				t.Errorf("a job was saved")
			}
		})
	}
}

func TestCreateJobTooLarge(t *testing.T) {
	env := newUploadEnv(t)
	f := validForm()
	f.file = append([]byte("%PDF-"), bytes.Repeat([]byte("x"), testMaxBytes)...)

	rec := env.post(t, f)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body: %s", rec.Code, rec.Body)
	}
	if code := decodeError(t, rec).Code; code != "file_too_large" {
		t.Errorf("code = %q, want file_too_large", code)
	}
	if files := env.storedFiles(t); len(files) != 0 {
		t.Errorf("files left behind: %v", files)
	}
}

func TestCreateJobSaveFailureRemovesFile(t *testing.T) {
	env := newUploadEnv(t)
	env.jobs.saveErr = errors.New("disk full")

	rec := env.post(t, validForm())

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if files := env.storedFiles(t); len(files) != 0 {
		t.Errorf("files left behind: %v", files)
	}
}

func TestCreateJobRejectsNonMultipart(t *testing.T) {
	env := newUploadEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(`{"printerId":"mono"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGetJobNotFound(t *testing.T) {
	env := newUploadEnv(t)
	rec := httptest.NewRecorder()

	env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/missing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if code := decodeError(t, rec).Code; code != "job_not_found" {
		t.Errorf("code = %q, want job_not_found", code)
	}
}

func TestCleanFilename(t *testing.T) {
	tests := map[string]string{
		"relatório.pdf":          "relatório.pdf",
		`C:\Users\raul\doc.pdf`:  "doc.pdf",
		"../../etc/passwd":       "passwd",
		"  spaced.pdf  ":         "spaced.pdf",
		"bad\x00name\n.pdf":      "badname.pdf",
		"":                       defaultFilename,
		"/":                      defaultFilename,
		strings.Repeat("é", 300): strings.Repeat("é", maxFilenameRunes),
	}
	for in, want := range tests {
		if got := cleanFilename(in); got != want {
			t.Errorf("cleanFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCancelJob(t *testing.T) {
	tests := []struct {
		name     string
		cancel   func(job.ID) (*job.Job, error)
		wantCode int
		wantErr  string
	}{
		{
			name: "cancelled",
			cancel: func(id job.ID) (*job.Job, error) {
				j := job.New(id, job.PrinterSnapshot{ID: "mono"}, job.Document{}, printer.Options{}, time.Now())
				return j, j.Cancel(time.Now())
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "not found",
			cancel:   func(job.ID) (*job.Job, error) { return nil, job.ErrNotFound },
			wantCode: http.StatusNotFound,
			wantErr:  "job_not_found",
		},
		{
			name: "too late",
			cancel: func(job.ID) (*job.Job, error) {
				return nil, &job.TransitionError{From: job.StatusCompleted, To: job.StatusCancelled}
			},
			wantCode: http.StatusConflict,
			wantErr:  "job_not_cancellable",
		},
		{
			name:     "provider error",
			cancel:   func(job.ID) (*job.Job, error) { return nil, errors.New("cups unreachable") },
			wantCode: http.StatusInternalServerError,
			wantErr:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newUploadEnv(t)
			env.worker.cancel = tt.cancel
			rec := httptest.NewRecorder()

			env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/jobs/j1/cancel", nil))

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantCode, rec.Body)
			}
			if tt.wantErr != "" {
				if code := decodeError(t, rec).Code; code != tt.wantErr {
					t.Errorf("code = %q, want %q", code, tt.wantErr)
				}
				return
			}
			var resp jobResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil || resp.Status != job.StatusCancelled {
				t.Errorf("response = %+v, %v; want a cancelled job", resp, err)
			}
		})
	}
}

func TestListJobs(t *testing.T) {
	env := newUploadEnv(t)
	queued := job.New("q", job.PrinterSnapshot{ID: "mono"}, job.Document{}, printer.Options{}, time.Now())
	cancelled := job.New("c", job.PrinterSnapshot{ID: "mono"}, job.Document{}, printer.Options{}, time.Now())
	cancelled.Cancel(time.Now())
	env.jobs.jobs = map[job.ID]*job.Job{"q": queued, "c": cancelled}

	get := func(query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/jobs"+query, nil))
		return rec
	}

	rec := get("?status=queued,printing")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	var body jobListResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Jobs) != 1 || body.Jobs[0].ID != "q" {
		t.Errorf("jobs = %+v, want only q", body.Jobs)
	}
	if env.jobs.lastLimit != defaultListLimit {
		t.Errorf("limit = %d, want the default %d", env.jobs.lastLimit, defaultListLimit)
	}

	if get("?limit=5").Code != http.StatusOK || env.jobs.lastLimit != 5 {
		t.Errorf("limit=5 not passed through (got %d)", env.jobs.lastLimit)
	}

	for _, bad := range []string{"?status=nope", "?limit=0", "?limit=1000", "?limit=x"} {
		if rec := get(bad); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", bad, rec.Code)
		}
	}

	// An empty result is an empty array, not null.
	env.jobs.jobs = map[job.ID]*job.Job{}
	if got := strings.TrimSpace(get("").Body.String()); got != `{"jobs":[]}` {
		t.Errorf("empty list body = %s", got)
	}
}
