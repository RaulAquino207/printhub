// Package httpapi exposes PrintHub over HTTP: routing, middleware and handlers.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Dependencies are everything the HTTP layer needs, passed explicitly by main.
type Dependencies struct {
	Logger *slog.Logger
	// CORSOrigins lists the browser origins allowed to call the API
	// cross-origin (e.g. the web app).
	CORSOrigins []string
	Printers    PrinterDirectory
}

// server holds the dependencies shared by handlers. Handlers that need them
// are methods on *server.
type server struct {
	logger   *slog.Logger
	printers PrinterDirectory
}

// NewRouter returns the root HTTP handler of the API.
func NewRouter(deps Dependencies) http.Handler {
	s := &server{logger: deps.Logger, printers: deps.Printers}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(logRequests(deps.Logger))
	r.Use(middleware.Recoverer)
	r.Use(allowOrigins(deps.CORSOrigins))

	r.Get("/healthz", handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/printers", s.listPrinters)
		r.Get("/printers/{id}", s.getPrinter)
	})

	return r
}

// logRequests logs one line per request after it has been handled.
func logRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
