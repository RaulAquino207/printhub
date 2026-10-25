package cups

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/OpenPrinting/goipp"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing"
	"github.com/RaulAquino207/printhub/apps/api/internal/worker"
)

// Compile-time checks: the CUPS provider fits both consumers.
var (
	_ printer.Provider = (*Provider)(nil)
	_ worker.Provider  = (*Provider)(nil)
)

func TestNewValidatesURL(t *testing.T) {
	for _, bad := range []string{"", "localhost:631", "ipp://host:631", "http://"} {
		if _, err := New(Options{URL: bad}); err == nil {
			t.Errorf("New(%q) succeeded, want an error", bad)
		}
	}
	p, err := New(Options{URL: "http://print-bridge:631"})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.ippURI("/printers/q"); got != "ipp://print-bridge:631/printers/q" {
		t.Errorf("ippURI = %q", got)
	}
}

func TestCapabilitiesFrom(t *testing.T) {
	a := attrs{
		goipp.MakeAttr("print-color-mode-supported", goipp.TagKeyword, goipp.String("monochrome"), goipp.String("color")),
		goipp.MakeAttr("media-supported", goipp.TagKeyword,
			goipp.String("iso_a4_210x297mm"), goipp.String("na_letter_8.5x11in"), goipp.String("om_custom_1x1in")),
		goipp.MakeAttr("orientation-requested-supported", goipp.TagEnum, goipp.Integer(3), goipp.Integer(4), goipp.Integer(5)),
		goipp.MakeAttr("sides-supported", goipp.TagKeyword, goipp.String("one-sided"), goipp.String("two-sided-long-edge")),
		goipp.MakeAttr("copies-supported", goipp.TagRange, goipp.Range{Lower: 1, Upper: 9999}),
		goipp.MakeAttr("page-ranges-supported", goipp.TagBoolean, goipp.Boolean(true)),
	}

	c := capabilitiesFrom(a)

	want := printer.Capabilities{
		ColorModes:   []printer.ColorMode{printer.ColorModeColor, printer.ColorModeMonochrome},
		PaperSizes:   []printer.PaperSize{"A4", "Letter"}, // unknown media ignored
		Orientations: []printer.Orientation{printer.OrientationPortrait, printer.OrientationLandscape},
		Duplex:       true,
		PageRanges:   true,
		MaxCopies:    9999,
	}
	if !slices.Equal(c.ColorModes, want.ColorModes) || !slices.Equal(c.PaperSizes, want.PaperSizes) ||
		!slices.Equal(c.Orientations, want.Orientations) || c.Duplex != want.Duplex ||
		c.PageRanges != want.PageRanges || c.MaxCopies != want.MaxCopies {
		t.Errorf("capabilities = %+v\nwant %+v", c, want)
	}
}

func TestCapabilitiesDefaults(t *testing.T) {
	// A queue that reports almost nothing: monochrome, one copy, both orientations.
	c := capabilitiesFrom(attrs{})

	if !slices.Equal(c.ColorModes, []printer.ColorMode{printer.ColorModeMonochrome}) ||
		c.MaxCopies != 1 || len(c.Orientations) != 2 || c.Duplex || c.PageRanges {
		t.Errorf("capabilities = %+v", c)
	}
}

func TestJobAttributes(t *testing.T) {
	got := attrs(jobAttributes(printer.Options{
		Copies:      3,
		ColorMode:   printer.ColorModeColor,
		PaperSize:   "Letter",
		Orientation: printer.OrientationLandscape,
		Duplex:      true,
		PageRanges:  "1-3, 7",
	}))

	if n, _ := got.int("copies"); n != 3 {
		t.Errorf("copies = %d", n)
	}
	for name, want := range map[string]string{
		"print-color-mode": "color",
		"media":            "na_letter_8.5x11in",
		"sides":            "two-sided-long-edge",
	} {
		if v := got.string(name); v != want {
			t.Errorf("%s = %q, want %q", name, v, want)
		}
	}
	if o, _ := got.int("orientation-requested"); o != orientationLandscape {
		t.Errorf("orientation-requested = %d", o)
	}
	ranges := got.values("page-ranges")
	if len(ranges) != 2 || ranges[0].V != (goipp.Range{Lower: 1, Upper: 3}) || ranges[1].V != (goipp.Range{Lower: 7, Upper: 7}) {
		t.Errorf("page-ranges = %v", ranges)
	}
}

// fakeCUPS answers every IPP request with the given status and attributes.
func fakeCUPS(t *testing.T, status goipp.Status, job goipp.Attributes) *Provider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req goipp.Message
		if err := req.Decode(r.Body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		io.Copy(io.Discard, r.Body) // the document, for Print-Job
		resp := goipp.NewResponse(goipp.DefaultVersion, status, req.RequestID)
		resp.Job = job
		w.Header().Set("Content-Type", goipp.ContentType)
		resp.Encode(w)
	}))
	t.Cleanup(srv.Close)
	p, err := New(Options{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestErrorMapping(t *testing.T) {
	ctx := context.Background()
	doc := func() io.Reader { return strings.NewReader("%PDF-1.7") }

	if _, err := fakeCUPS(t, goipp.StatusErrorNotAcceptingJobs, nil).Submit(ctx, "q", doc(), printer.Options{}); !errors.Is(err, printing.ErrPrinterUnavailable) {
		t.Errorf("not accepting jobs: err = %v", err)
	}
	if _, err := fakeCUPS(t, goipp.StatusErrorNotFound, nil).JobStatus(ctx, "1"); !errors.Is(err, printing.ErrUnknownJob) {
		t.Errorf("job not found: err = %v", err)
	}
	if err := fakeCUPS(t, goipp.StatusErrorNotPossible, nil).Cancel(ctx, "1"); !errors.Is(err, printing.ErrJobFinished) {
		t.Errorf("cancel finished job: err = %v", err)
	}
	if _, err := fakeCUPS(t, goipp.StatusOk, nil).JobStatus(ctx, "not-a-number"); !errors.Is(err, printing.ErrUnknownJob) {
		t.Errorf("malformed job id: err = %v", err)
	}
	if _, err := fakeCUPS(t, goipp.StatusErrorForbidden, nil).Submit(ctx, "q", doc(), printer.Options{}); err == nil || errors.Is(err, printing.ErrPrinterUnavailable) {
		t.Errorf("forbidden: err = %v, want a generic error", err)
	}
}

func TestJobStateMapping(t *testing.T) {
	tests := map[int]printing.JobState{
		3: printing.JobPending, 4: printing.JobPending,
		5: printing.JobProcessing, 6: printing.JobProcessing,
		7: printing.JobCancelled, 8: printing.JobFailed, 9: printing.JobCompleted,
	}
	for state, want := range tests {
		p := fakeCUPS(t, goipp.StatusOk, goipp.Attributes{
			goipp.MakeAttr("job-state", goipp.TagEnum, goipp.Integer(state)),
			goipp.MakeAttr("job-state-message", goipp.TagText, goipp.String("details")),
		})
		st, err := p.JobStatus(context.Background(), "1")
		if err != nil || st.State != want || st.Message != "details" {
			t.Errorf("job-state %d: %+v, %v; want %s", state, st, err, want)
		}
	}
}

func TestSubmitReturnsJobID(t *testing.T) {
	p := fakeCUPS(t, goipp.StatusOk, goipp.Attributes{goipp.MakeAttr("job-id", goipp.TagInteger, goipp.Integer(42))})

	id, err := p.Submit(context.Background(), "q", strings.NewReader("%PDF-1.7"), printer.Options{Copies: 1})
	if err != nil || id != "42" {
		t.Errorf("Submit = %q, %v; want 42", id, err)
	}
}
