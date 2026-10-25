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
	Jobs        JobStore
	Documents   DocumentStore
	Worker      JobRunner
	Auth        Authenticator
	// CookieSecure marks the session cookie HTTPS-only. Enable it whenever
	// the API is served over HTTPS (always, outside local development).
	CookieSecure bool
}

// server holds the dependencies shared by handlers. Handlers that need them
// are methods on *server.
type server struct {
	logger       *slog.Logger
	printers     PrinterDirectory
	jobs         JobStore
	documents    DocumentStore
	worker       JobRunner
	auth         Authenticator
	cookieSecure bool
}

// NewRouter returns the root HTTP handler of the API.
func NewRouter(deps Dependencies) http.Handler {
	s := &server{
		logger:    deps.Logger,
		printers:  deps.Printers,
		jobs:      deps.Jobs,
		documents: deps.Documents,
		worker:    deps.Worker,
		auth:      deps.Auth,

		cookieSecure: deps.CookieSecure,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(logRequests(deps.Logger))
	r.Use(middleware.Recoverer)
	r.Use(allowOrigins(deps.CORSOrigins))

	r.Get("/healthz", handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(checkCSRF(deps.CORSOrigins))

		r.Post("/auth/login", s.login)

		// Everything else requires a session.
		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)

			r.Post("/auth/logout", s.logout)
			r.Get("/auth/me", s.me)

			r.Get("/printers", s.listPrinters)
			r.Get("/printers/{id}", s.getPrinter)
			r.Get("/jobs", s.listJobs)
			r.Post("/jobs", s.createJob)
			r.Get("/jobs/{id}", s.getJob)
			r.Post("/jobs/{id}/cancel", s.cancelJob)
		})
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
