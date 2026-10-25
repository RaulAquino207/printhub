//go:build integration

// Integration tests against a real CUPS server. They run only with the
// "integration" build tag and the development CUPS container:
//
//	moon run api:cups-up
//	moon run api:test-integration
//
// The container's queues print to /dev/null: passing tests prove the IPP
// integration, not that anything can be printed on paper.
package cups

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing"
)

const testPDF = "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj 3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 595 842]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n"

func newIntegrationProvider(t *testing.T) *Provider {
	t.Helper()
	url := os.Getenv("PRINTHUB_TEST_CUPS_URL")
	if url == "" {
		url = "http://localhost:6631"
	}
	p, err := New(Options{URL: url, Simulated: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Printers(context.Background()); err != nil {
		t.Fatalf("CUPS not reachable at %s (start it with: moon run api:cups-up): %v", url, err)
	}
	return p
}

var mono = printer.Options{Copies: 1, ColorMode: printer.ColorModeMonochrome, PaperSize: "A4", Orientation: printer.OrientationPortrait}

func waitForState(t *testing.T, p *Provider, id string, want printing.JobState) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		st, err := p.JobStatus(context.Background(), id)
		if err != nil {
			t.Fatalf("JobStatus(%s): %v", id, err)
		}
		if st.State == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s is %s, want %s", id, st.State, want)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestDiscoveryAndCapabilities(t *testing.T) {
	p := newIntegrationProvider(t)
	ctx := context.Background()

	configs, err := p.Configs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var queues []string
	for _, c := range configs {
		queues = append(queues, c.Destination)
		if !c.Simulated || string(c.ID) != c.Destination || c.Name == "" {
			t.Errorf("config = %+v", c)
		}
	}
	for _, q := range []string{"virtual-mono", "virtual-color", "virtual-paused"} {
		if !slices.Contains(queues, q) {
			t.Errorf("queue %s not discovered (got %v)", q, queues)
		}
	}

	tests := []struct {
		queue      string
		status     printer.Status
		colorModes []printer.ColorMode
		duplex     bool
	}{
		{"virtual-mono", printer.StatusOnline, []printer.ColorMode{printer.ColorModeMonochrome}, false},
		{"virtual-color", printer.StatusOnline, []printer.ColorMode{printer.ColorModeColor, printer.ColorModeMonochrome}, true},
		{"virtual-paused", printer.StatusOffline, []printer.ColorMode{printer.ColorModeMonochrome}, false},
	}
	for _, tt := range tests {
		t.Run(tt.queue, func(t *testing.T) {
			status, err := p.Status(ctx, tt.queue)
			if err != nil || status != tt.status {
				t.Errorf("Status = %s, %v; want %s", status, err, tt.status)
			}
			c, err := p.Capabilities(ctx, tt.queue)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(c.ColorModes, tt.colorModes) || c.Duplex != tt.duplex {
				t.Errorf("capabilities = %+v", c)
			}
			if !slices.Contains(c.PaperSizes, "A4") || !slices.Contains(c.PaperSizes, "Letter") {
				t.Errorf("paper sizes = %v, want A4 and Letter", c.PaperSizes)
			}
			if c.MaxCopies < 1 || len(c.Orientations) == 0 {
				t.Errorf("capabilities = %+v", c)
			}
		})
	}
}

func TestPrintJobCompletes(t *testing.T) {
	p := newIntegrationProvider(t)
	opts := mono
	opts.Copies = 2
	opts.PageRanges = "1"

	id, err := p.Submit(context.Background(), "virtual-mono", strings.NewReader(testPDF), opts)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForState(t, p, id, printing.JobCompleted)
}

func TestCancelPendingJob(t *testing.T) {
	p := newIntegrationProvider(t)
	ctx := context.Background()

	// The paused queue keeps jobs pending forever.
	id, err := p.Submit(ctx, "virtual-paused", strings.NewReader(testPDF), mono)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForState(t, p, id, printing.JobPending)

	if err := p.Cancel(ctx, id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForState(t, p, id, printing.JobCancelled)

	if err := p.Cancel(ctx, id); !errors.Is(err, printing.ErrJobFinished) {
		t.Errorf("second Cancel err = %v, want ErrJobFinished", err)
	}
}

func TestErrors(t *testing.T) {
	p := newIntegrationProvider(t)
	ctx := context.Background()

	if _, err := p.Submit(ctx, "no-such-queue", strings.NewReader(testPDF), mono); !errors.Is(err, printing.ErrPrinterUnavailable) {
		t.Errorf("Submit to missing queue err = %v, want ErrPrinterUnavailable", err)
	}
	if _, err := p.JobStatus(ctx, "999999"); !errors.Is(err, printing.ErrUnknownJob) {
		t.Errorf("JobStatus of unknown job err = %v, want ErrUnknownJob", err)
	}
	if err := p.Cancel(ctx, "999999"); !errors.Is(err, printing.ErrUnknownJob) {
		t.Errorf("Cancel of unknown job err = %v, want ErrUnknownJob", err)
	}
}
