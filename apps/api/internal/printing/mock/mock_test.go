package mock

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing"
)

// Compile-time check: *Provider must satisfy printer.Provider. If a method is
// missing or has the wrong signature, this line stops the build.
var _ printer.Provider = (*Provider)(nil)

func TestProvider(t *testing.T) {
	ctx := context.Background()
	p := New(Printer{
		Destination:  "q",
		Status:       printer.StatusOffline,
		Capabilities: printer.Capabilities{MaxCopies: 5},
	})

	status, err := p.Status(ctx, "q")
	if err != nil || status != printer.StatusOffline {
		t.Errorf("Status(q) = %q, %v; want offline, nil", status, err)
	}

	caps, err := p.Capabilities(ctx, "q")
	if err != nil || caps.MaxCopies != 5 {
		t.Errorf("Capabilities(q) = %+v, %v; want MaxCopies 5, nil", caps, err)
	}

	if _, err := p.Status(ctx, "missing"); !errors.Is(err, ErrUnknownDestination) {
		t.Errorf("Status(missing) err = %v, want ErrUnknownDestination", err)
	}
	if _, err := p.Capabilities(ctx, "missing"); !errors.Is(err, ErrUnknownDestination) {
		t.Errorf("Capabilities(missing) err = %v, want ErrUnknownDestination", err)
	}
}

func TestDemoConfigsMatchProvider(t *testing.T) {
	configs, provider := Demo()

	for _, cfg := range configs {
		if !cfg.Simulated {
			t.Errorf("%s: demo printer must be marked as simulated", cfg.ID)
		}
		if _, err := provider.Status(context.Background(), cfg.Destination); err != nil {
			t.Errorf("%s: destination %q not simulated: %v", cfg.ID, cfg.Destination, err)
		}
	}
}

// Stored jobs reference printers by ID, so these IDs must survive restarts and
// refactors. Changing one orphans the jobs that point to it: treat any change
// here as a data migration, not a rename.
func TestDemoIDsAreStable(t *testing.T) {
	configs, _ := Demo()

	var ids []printer.ID
	for _, cfg := range configs {
		ids = append(ids, cfg.ID)
	}

	want := []printer.ID{"mono-laser", "color-laser", "inkjet", "faulty-laser"}
	if !slices.Equal(ids, want) {
		t.Errorf("demo printer IDs = %v, want %v", ids, want)
	}
}

// fakeClock is a clock tests move forward by hand.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newJobProvider(t *testing.T) (*Provider, *fakeClock) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)}
	p := New(
		Printer{Destination: "ok", Status: printer.StatusOnline},
		Printer{Destination: "jam", Status: printer.StatusOnline, FailJobs: true},
		Printer{Destination: "off", Status: printer.StatusOffline},
	)
	p.SetClock(clock.now)
	p.SetTiming(Timing{Queue: time.Second, Print: 2 * time.Second})
	return p, clock
}

func submit(t *testing.T, p *Provider, dest string) string {
	t.Helper()
	id, err := p.Submit(context.Background(), dest, strings.NewReader("%PDF-1.7"), printer.Options{})
	if err != nil {
		t.Fatalf("Submit(%s): %v", dest, err)
	}
	return id
}

func stateOf(t *testing.T, p *Provider, id string) printing.JobState {
	t.Helper()
	st, err := p.JobStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("JobStatus(%s): %v", id, err)
	}
	return st.State
}

func TestJobTimeline(t *testing.T) {
	p, clock := newJobProvider(t)
	id := submit(t, p, "ok")

	steps := []struct {
		after time.Duration
		want  printing.JobState
	}{
		{0, printing.JobPending},
		{time.Second, printing.JobProcessing},
		{2 * time.Second, printing.JobCompleted},
	}
	for _, s := range steps {
		clock.advance(s.after)
		if got := stateOf(t, p, id); got != s.want {
			t.Errorf("after +%v: state = %s, want %s", s.after, got, s.want)
		}
	}
}

func TestFailingPrinter(t *testing.T) {
	p, clock := newJobProvider(t)
	id := submit(t, p, "jam")

	clock.advance(time.Second)
	st, err := p.JobStatus(context.Background(), id)
	if err != nil || st.State != printing.JobFailed || st.Message == "" {
		t.Errorf("status = %+v, %v; want failed with a message", st, err)
	}
}

func TestSubmitToOfflinePrinter(t *testing.T) {
	p, _ := newJobProvider(t)
	_, err := p.Submit(context.Background(), "off", strings.NewReader(""), printer.Options{})
	if !errors.Is(err, printing.ErrPrinterUnavailable) {
		t.Errorf("err = %v, want ErrPrinterUnavailable", err)
	}
}

func TestCancel(t *testing.T) {
	p, clock := newJobProvider(t)
	ctx := context.Background()

	pending := submit(t, p, "ok")
	if err := p.Cancel(ctx, pending); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := stateOf(t, p, pending); got != printing.JobCancelled {
		t.Errorf("state = %s, want cancelled", got)
	}

	done := submit(t, p, "ok")
	clock.advance(time.Hour)
	if err := p.Cancel(ctx, done); !errors.Is(err, printing.ErrJobFinished) {
		t.Errorf("Cancel finished job: err = %v, want ErrJobFinished", err)
	}

	if err := p.Cancel(ctx, "mock-999"); !errors.Is(err, printing.ErrUnknownJob) {
		t.Errorf("Cancel unknown job: err = %v, want ErrUnknownJob", err)
	}
	if _, err := p.JobStatus(ctx, "mock-999"); !errors.Is(err, printing.ErrUnknownJob) {
		t.Errorf("JobStatus unknown job: err = %v, want ErrUnknownJob", err)
	}
}
