package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

type fakeDirectory struct {
	printers []printer.Printer
	err      error
}

func (f fakeDirectory) List(context.Context) ([]printer.Printer, error) {
	return f.printers, f.err
}

func (f fakeDirectory) Get(_ context.Context, id printer.ID) (printer.Printer, error) {
	if f.err != nil {
		return printer.Printer{}, f.err
	}
	for _, p := range f.printers {
		if p.ID == id {
			return p, nil
		}
	}
	return printer.Printer{}, printer.ErrNotFound
}

func newTestRouter(dir PrinterDirectory) http.Handler {
	return signedIn(NewRouter(Dependencies{
		Logger:      slog.New(slog.DiscardHandler),
		CORSOrigins: []string{testOrigin},
		Printers:    dir,
		Auth:        fakeAuth{},
	}))
}

func serve(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

var testPrinters = []printer.Printer{
	{
		Config: printer.Config{ID: "mono", Name: "Mono", Model: "M", Simulated: true},
		Status: printer.StatusOnline,
		Capabilities: printer.Capabilities{
			ColorModes: []printer.ColorMode{printer.ColorModeMonochrome},
			PaperSizes: []printer.PaperSize{"A4"},
			MaxCopies:  10,
		},
	},
	// Unknown status: no capabilities reported.
	{Config: printer.Config{ID: "lost", Name: "Lost"}, Status: printer.StatusUnknown},
}

func TestListPrinters(t *testing.T) {
	rec := serve(t, newTestRouter(fakeDirectory{printers: testPrinters}), "/api/v1/printers")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body printerListResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Printers) != 2 {
		t.Fatalf("got %d printers, want 2", len(body.Printers))
	}
	mono := body.Printers[0]
	if mono.ID != "mono" || !mono.Simulated || mono.Status != "online" || mono.Capabilities.MaxCopies != 10 {
		t.Errorf("mono = %+v", mono)
	}
}

func TestListPrintersEncodesEmptyListsAsArrays(t *testing.T) {
	rec := serve(t, newTestRouter(fakeDirectory{printers: testPrinters}), "/api/v1/printers")

	// Decode generically to see the raw JSON types.
	var body struct {
		Printers []map[string]any `json:"printers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	caps := body.Printers[1]["capabilities"].(map[string]any)
	for _, field := range []string{"colorModes", "paperSizes", "orientations"} {
		if _, ok := caps[field].([]any); !ok {
			t.Errorf("%s = %#v, want an empty array", field, caps[field])
		}
	}
}

func TestListPrintersError(t *testing.T) {
	rec := serve(t, newTestRouter(fakeDirectory{err: errors.New("boom")}), "/api/v1/printers")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	assertErrorCode(t, rec, "internal_error")
}

func TestGetPrinter(t *testing.T) {
	router := newTestRouter(fakeDirectory{printers: testPrinters})

	rec := serve(t, router, "/api/v1/printers/mono")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var p printerResponse
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ID != "mono" {
		t.Errorf("id = %q, want mono", p.ID)
	}

	rec = serve(t, router, "/api/v1/printers/missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec, "printer_not_found")
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != want {
		t.Errorf("error code = %q, want %q", body.Error.Code, want)
	}
}
