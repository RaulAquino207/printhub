package store

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/job"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

func mustSub(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

// t0 is in UTC without a monotonic reading, like the times read back from
// the database, so reflect.DeepEqual can compare whole jobs.
var t0 = time.Date(2026, 10, 25, 12, 0, 0, 123456789, time.UTC)

func at(sec int) time.Time { return t0.Add(time.Duration(sec) * time.Second) }

func newJob(id job.ID, created time.Time) *job.Job {
	return job.New(id,
		job.PrinterSnapshot{ID: "color-laser", Name: "Laser colorida", Model: "Simulated color laser", Simulated: true},
		job.Document{Name: "relatório.pdf", SizeBytes: 48213},
		printer.Options{
			Copies:      2,
			ColorMode:   printer.ColorModeColor,
			PaperSize:   "A4",
			Orientation: printer.OrientationLandscape,
			Duplex:      true,
			PageRanges:  "1-3, 5",
		},
		created,
	)
}

func TestOpenConfiguresSQLite(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "printhub.db"))
	defer s.Close()

	pragmas := map[string]string{
		"journal_mode": "wal",
		"busy_timeout": "5000",
		"foreign_keys": "1",
		"synchronous":  "1", // NORMAL
	}
	for name, want := range pragmas {
		var got string
		if err := s.db.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", name, err)
		}
		if got != want {
			t.Errorf("PRAGMA %s = %q, want %q", name, got, want)
		}
	}
}

// TestJobSurvivesReopen is the real persistence check: write, close the
// database, open the same file again, read back.
func TestJobSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "printhub.db")

	want := newJob("job-1", t0)
	steps := []func() error{
		func() error { return want.StartProcessing(at(1)) },
		func() error { return want.MarkSubmitted("cups-42", at(2)) },
		func() error { return want.Fail("printer went offline", at(3)) },
	}

	s := openTestStore(t, path)
	if err := s.SaveJob(ctx, want); err != nil {
		t.Fatalf("SaveJob (created): %v", err)
	}
	// Save after every transition, as the worker will.
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveJob(ctx, want); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened := openTestStore(t, path)
	defer reopened.Close()

	got, err := reopened.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob after reopen: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("job after reopen:\n got  %+v\n want %+v", got, want)
	}
}

func TestSaveJobKeepsImmutableFields(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, filepath.Join(t.TempDir(), "printhub.db"))
	defer s.Close()

	j := newJob("job-1", t0)
	if err := s.SaveJob(ctx, j); err != nil {
		t.Fatal(err)
	}

	// Even if the in-memory snapshot were changed, the stored one must not be.
	j.Printer.Name = "Renamed"
	if err := j.StartProcessing(at(1)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveJob(ctx, j); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Printer.Name != "Laser colorida" {
		t.Errorf("printer name = %q, want the original snapshot", got.Printer.Name)
	}
	if got.Status != job.StatusProcessing || len(got.History) != 2 {
		t.Errorf("status %s with %d transitions, want processing with 2", got.Status, len(got.History))
	}
}

func TestGetJobNotFound(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "printhub.db"))
	defer s.Close()

	if _, err := s.GetJob(context.Background(), "missing"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("err = %v, want ErrJobNotFound", err)
	}
}

func TestListUnfinished(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, filepath.Join(t.TempDir(), "printhub.db"))
	defer s.Close()

	// One job per status, created a second apart.
	jobs := map[job.ID]func(*job.Job, time.Time) error{
		"queued":     func(*job.Job, time.Time) error { return nil },
		"processing": func(j *job.Job, t time.Time) error { return j.StartProcessing(t) },
		"submitted": func(j *job.Job, t time.Time) error {
			return errors.Join(j.StartProcessing(t), j.MarkSubmitted("cups-1", t))
		},
		"completed": func(j *job.Job, t time.Time) error {
			return errors.Join(j.StartProcessing(t), j.MarkSubmitted("cups-2", t), j.Complete(t))
		},
		"cancelled": func(j *job.Job, t time.Time) error { return j.Cancel(t) },
	}
	order := []job.ID{"submitted", "queued", "completed", "processing", "cancelled"}
	for i, id := range order {
		j := newJob(id, at(i))
		if err := jobs[id](j, at(i)); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if err := s.SaveJob(ctx, j); err != nil {
			t.Fatal(err)
		}
	}

	unfinished, err := s.ListUnfinished(ctx)
	if err != nil {
		t.Fatalf("ListUnfinished: %v", err)
	}

	var ids []job.ID
	for _, j := range unfinished {
		ids = append(ids, j.ID)
		if len(j.History) == 0 {
			t.Errorf("%s: history not loaded", j.ID)
		}
	}
	// Oldest first, terminal jobs excluded.
	want := []job.ID{"submitted", "queued", "processing"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("unfinished = %v, want %v", ids, want)
	}
	if unfinished[0].ProviderJobID != "cups-1" {
		t.Errorf("submitted job lost its provider job ID: %q", unfinished[0].ProviderJobID)
	}
}
