package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
)

// PrinterDirectory is what the HTTP layer needs to read printers.
// *printer.Registry satisfies it.
type PrinterDirectory interface {
	List(ctx context.Context) ([]printer.Printer, error)
	Get(ctx context.Context, id printer.ID) (printer.Printer, error)
}

// printerResponse is the API contract for a printer. It is separate from
// printer.Printer so the domain can change without silently changing the JSON.
type printerResponse struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Model        string               `json:"model"`
	Status       string               `json:"status"`
	Simulated    bool                 `json:"simulated"`
	Capabilities capabilitiesResponse `json:"capabilities"`
}

type capabilitiesResponse struct {
	ColorModes   []printer.ColorMode   `json:"colorModes"`
	PaperSizes   []printer.PaperSize   `json:"paperSizes"`
	Orientations []printer.Orientation `json:"orientations"`
	Duplex       bool                  `json:"duplex"`
	PageRanges   bool                  `json:"pageRanges"`
	MaxCopies    int                   `json:"maxCopies"`
}

type printerListResponse struct {
	Printers []printerResponse `json:"printers"`
}

func toPrinterResponse(p printer.Printer) printerResponse {
	c := p.Capabilities
	return printerResponse{
		ID:        string(p.ID),
		Name:      p.Name,
		Model:     p.Model,
		Status:    string(p.Status),
		Simulated: p.Simulated,
		Capabilities: capabilitiesResponse{
			ColorModes:   nonNil(c.ColorModes),
			PaperSizes:   nonNil(c.PaperSizes),
			Orientations: nonNil(c.Orientations),
			Duplex:       c.Duplex,
			PageRanges:   c.PageRanges,
			MaxCopies:    c.MaxCopies,
		},
	}
}

// nonNil turns a nil slice into an empty one, so it encodes as [] instead of null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (s *server) listPrinters(w http.ResponseWriter, r *http.Request) {
	printers, err := s.printers.List(r.Context())
	if err != nil {
		s.logger.Error("list printers", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not list printers")
		return
	}

	resp := printerListResponse{Printers: make([]printerResponse, 0, len(printers))}
	for _, p := range printers {
		resp.Printers = append(resp.Printers, toPrinterResponse(p))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) getPrinter(w http.ResponseWriter, r *http.Request) {
	id := printer.ID(chi.URLParam(r, "id"))

	p, err := s.printers.Get(r.Context(), id)
	if errors.Is(err, printer.ErrNotFound) {
		writeError(w, http.StatusNotFound, "printer_not_found", "printer not found")
		return
	}
	if err != nil {
		s.logger.Error("get printer", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not get printer")
		return
	}

	writeJSON(w, http.StatusOK, toPrinterResponse(p))
}
