package job

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

var allStatuses = []Status{
	StatusQueued, StatusProcessing, StatusSubmitted, StatusPrinting,
	StatusCompleted, StatusFailed, StatusCancelled,
}

// TestCanTransition checks every (from, to) pair against the expected
// lifecycle, so adding or removing a transition is always a deliberate change.
func TestCanTransition(t *testing.T) {
	allowed := map[[2]Status]bool{
		{StatusQueued, StatusProcessing}:    true,
		{StatusQueued, StatusFailed}:        true,
		{StatusQueued, StatusCancelled}:     true,
		{StatusProcessing, StatusSubmitted}: true,
		{StatusProcessing, StatusFailed}:    true,
		{StatusSubmitted, StatusPrinting}:   true,
		{StatusSubmitted, StatusCompleted}:  true,
		{StatusSubmitted, StatusFailed}:     true,
		{StatusSubmitted, StatusCancelled}:  true,
		{StatusPrinting, StatusCompleted}:   true,
		{StatusPrinting, StatusFailed}:      true,
		{StatusPrinting, StatusCancelled}:   true,
	}

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			want := allowed[[2]Status{from, to}]
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestTerminal(t *testing.T) {
	terminal := map[Status]bool{StatusCompleted: true, StatusFailed: true, StatusCancelled: true}
	for _, s := range allStatuses {
		if got := s.Terminal(); got != terminal[s] {
			t.Errorf("%s.Terminal() = %v, want %v", s, got, terminal[s])
		}
	}
}

func TestUnfinishedStatuses(t *testing.T) {
	unfinished := UnfinishedStatuses()
	for _, s := range allStatuses {
		if got, want := slices.Contains(unfinished, s), !s.Terminal(); got != want {
			t.Errorf("%s in UnfinishedStatuses() = %v, want %v", s, got, want)
		}
	}
}

func newTestJob(t0 time.Time) *Job {
	p := PrinterSnapshot{ID: "mono-laser", Name: "Laser P&B", Model: "Simulated monochrome laser", Simulated: true}
	return New("job-1", p, Document{Name: "a.pdf", SizeBytes: 1024}, printer.Options{Copies: 1}, t0)
}

func TestHappyPath(t *testing.T) {
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return t0.Add(time.Duration(sec) * time.Second) }

	j := newTestJob(t0)
	steps := []struct {
		name string
		do   func() error
	}{
		{"start processing", func() error { return j.StartProcessing(at(1)) }},
		{"submitted", func() error { return j.MarkSubmitted("cups-42", at(2)) }},
		{"printing", func() error { return j.MarkPrinting(at(3)) }},
		{"complete", func() error { return j.Complete(at(4)) }},
	}
	for _, s := range steps {
		if err := s.do(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
	}

	if j.Status != StatusCompleted {
		t.Errorf("Status = %s, want completed", j.Status)
	}
	if j.ProviderJobID != "cups-42" {
		t.Errorf("ProviderJobID = %q, want cups-42", j.ProviderJobID)
	}
	if !j.CreatedAt().Equal(t0) || !j.UpdatedAt().Equal(at(4)) {
		t.Errorf("CreatedAt = %v, UpdatedAt = %v", j.CreatedAt(), j.UpdatedAt())
	}

	wantHistory := []Status{StatusQueued, StatusProcessing, StatusSubmitted, StatusPrinting, StatusCompleted}
	if len(j.History) != len(wantHistory) {
		t.Fatalf("history has %d entries, want %d", len(j.History), len(wantHistory))
	}
	for i, tr := range j.History {
		if tr.Status != wantHistory[i] || !tr.At.Equal(at(i)) {
			t.Errorf("History[%d] = %+v, want %s at %v", i, tr, wantHistory[i], at(i))
		}
	}
}

func TestFailRecordsReason(t *testing.T) {
	t0 := time.Now()
	j := newTestJob(t0)

	if err := j.Fail("printer offline", t0); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if j.Status != StatusFailed || j.FailureReason != "printer offline" {
		t.Errorf("job = %s %q, want failed with reason", j.Status, j.FailureReason)
	}
}

func TestInvalidTransitionLeavesJobUnchanged(t *testing.T) {
	t0 := time.Now()
	j := newTestJob(t0)
	if err := j.Cancel(t0); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	err := j.StartProcessing(t0)

	// errors.Is matches the sentinel through TransitionError.Unwrap.
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	// errors.As extracts the concrete type to read its fields.
	var te *TransitionError
	if !errors.As(err, &te) || te.From != StatusCancelled || te.To != StatusProcessing {
		t.Errorf("TransitionError = %+v, want cancelled → processing", te)
	}
	if j.Status != StatusCancelled || len(j.History) != 2 {
		t.Errorf("job changed after a rejected transition: %s, %d history entries", j.Status, len(j.History))
	}
}

func TestProcessingCannotBeCancelled(t *testing.T) {
	t0 := time.Now()
	j := newTestJob(t0)
	if err := j.StartProcessing(t0); err != nil {
		t.Fatalf("StartProcessing: %v", err)
	}

	if err := j.Cancel(t0); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("Cancel while processing: err = %v, want ErrInvalidTransition", err)
	}
}

func TestMarkSubmittedRejectedKeepsProviderJobID(t *testing.T) {
	t0 := time.Now()
	j := newTestJob(t0)

	if err := j.MarkSubmitted("cups-1", t0); err == nil {
		t.Fatal("MarkSubmitted from queued: want an error")
	}
	if j.ProviderJobID != "" {
		t.Errorf("ProviderJobID = %q, want empty after a rejected transition", j.ProviderJobID)
	}
}

func TestSnapshotOf(t *testing.T) {
	p := printer.Printer{Config: printer.Config{ID: "x", Name: "X", Model: "M", Destination: "q", Simulated: true}}

	got := SnapshotOf(p)

	want := PrinterSnapshot{ID: "x", Name: "X", Model: "M", Simulated: true}
	if got != want {
		t.Errorf("SnapshotOf() = %+v, want %+v", got, want)
	}
}
