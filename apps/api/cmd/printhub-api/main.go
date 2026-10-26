// Command printhub-api runs the PrintHub HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
	"github.com/RaulAquino207/printhub/apps/api/internal/document"
	"github.com/RaulAquino207/printhub/apps/api/internal/httpapi"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing/cups"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing/mock"
	"github.com/RaulAquino207/printhub/apps/api/internal/store"
	"github.com/RaulAquino207/printhub/apps/api/internal/worker"
)

const (
	// shutdownTimeout bounds how long in-flight requests may take to finish on shutdown.
	shutdownTimeout = 10 * time.Second
	// workerInterval is how often the worker polls jobs when nothing wakes it.
	workerInterval = time.Second
)

type config struct {
	// addr is the TCP address to listen on. The default binds to localhost
	// only, so the API is not reachable from the local network during development.
	addr string
	// corsOrigins are the browser origins allowed to call the API (ADR 0003).
	corsOrigins []string
	// dataDir holds the SQLite database and the uploaded documents. Keep it
	// outside any web root and, in containers, on a persistent volume.
	dataDir string
	// maxUploadBytes is the largest PDF accepted.
	maxUploadBytes int64
	// cookieSecure sends the session cookie over HTTPS only. Off by default
	// for http://localhost; must be on in production.
	cookieSecure bool
	// provider is "mock" (simulated printers, the default) or "cups".
	provider string
	// cupsURL is the CUPS server, e.g. http://print-bridge:631.
	cupsURL string
	// cupsSimulated marks every CUPS printer as simulated, e.g. the dev
	// container whose queues print to /dev/null.
	cupsSimulated bool
}

func loadConfig() (config, error) {
	origins := splitList(envOr("PRINTHUB_CORS_ALLOWED_ORIGINS", "http://localhost:3000"))
	if slices.Contains(origins, "*") {
		return config{}, errors.New("PRINTHUB_CORS_ALLOWED_ORIGINS must list explicit origins, not *")
	}

	maxUploadMB, err := strconv.Atoi(envOr("PRINTHUB_MAX_UPLOAD_MB", "50"))
	if err != nil || maxUploadMB < 1 {
		return config{}, fmt.Errorf("PRINTHUB_MAX_UPLOAD_MB must be a positive integer, got %q", os.Getenv("PRINTHUB_MAX_UPLOAD_MB"))
	}

	provider := envOr("PRINTHUB_PRINTING_PROVIDER", "mock")
	cupsURL := os.Getenv("PRINTHUB_CUPS_URL")
	switch {
	case provider != "mock" && provider != "cups":
		return config{}, fmt.Errorf("PRINTHUB_PRINTING_PROVIDER must be mock or cups, got %q", provider)
	case provider == "cups" && cupsURL == "":
		return config{}, errors.New("PRINTHUB_CUPS_URL is required with PRINTHUB_PRINTING_PROVIDER=cups")
	}

	return config{
		addr:           envOr("PRINTHUB_API_ADDR", "localhost:8080"),
		corsOrigins:    origins,
		dataDir:        envOr("PRINTHUB_DATA_DIR", "data"),
		maxUploadBytes: int64(maxUploadMB) << 20,
		cookieSecure:   os.Getenv("PRINTHUB_COOKIE_SECURE") == "true",
		provider:       provider,
		cupsURL:        cupsURL,
		cupsSimulated:  os.Getenv("PRINTHUB_CUPS_SIMULATED") == "true",
	}, nil
}

// splitList parses a comma-separated value, ignoring blanks and surrounding spaces.
func splitList(s string) []string {
	var items []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// printingProvider is everything the API needs from a provider. Composing
// the two consumers' interfaces states that one value must satisfy both.
type printingProvider interface {
	printer.Provider
	worker.Provider
}

// newProvider returns the configured provider and its printers. With CUPS,
// the printers are the server's queues, discovered once at startup.
func newProvider(ctx context.Context, cfg config) ([]printer.Config, printingProvider, error) {
	if cfg.provider == "mock" {
		configs, p := mock.Demo()
		return configs, p, nil
	}
	p, err := cups.New(cups.Options{URL: cfg.cupsURL, Simulated: cfg.cupsSimulated})
	if err != nil {
		return nil, nil, err
	}
	configs, err := p.Configs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("discover CUPS printers at %s: %w", cfg.cupsURL, err)
	}
	return configs, p, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1:], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Info by default; PRINTHUB_LOG_LEVEL=debug also shows every polling request.
	var level slog.Level
	if err := level.UnmarshalText([]byte(envOr("PRINTHUB_LOG_LEVEL", "info"))); err != nil {
		fmt.Fprintln(os.Stderr, "PRINTHUB_LOG_LEVEL:", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	if err := run(logger); err != nil {
		logger.Error("api stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM (sent by Docker, air, systemd).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 0o700: only the user running the API can read the database.
	if err := os.MkdirAll(cfg.dataDir, 0o700); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	dbPath := filepath.Join(cfg.dataDir, "printhub.db")
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("database ready", "path", dbPath)

	sessions := auth.NewService(db)
	if n, err := sessions.DeleteExpiredSessions(ctx); err != nil {
		return err
	} else if n > 0 {
		logger.Info("removed expired sessions", "count", n)
	}

	documents, err := document.NewStorage(filepath.Join(cfg.dataDir, "documents"), cfg.maxUploadBytes)
	if err != nil {
		return err
	}

	printerConfigs, provider, err := newProvider(ctx, cfg)
	if err != nil {
		return err
	}
	logger.Info("printing provider ready", "provider", cfg.provider, "printers", len(printerConfigs))
	printers := printer.NewRegistry(printerConfigs, provider)

	jobs := worker.New(worker.Config{
		Store:     db,
		Printers:  printers,
		Provider:  provider,
		Documents: documents,
		Logger:    logger,
		Interval:  workerInterval,
	})

	// Before accepting uploads, so the cleanup never races with one.
	if err := jobs.CleanDocuments(ctx); err != nil {
		return fmt.Errorf("clean documents: %w", err)
	}

	// Jobs left unfinished by the last run are recovered by the worker's
	// first pass.
	unfinished, err := db.ListUnfinished(ctx)
	if err != nil {
		return err
	}
	if len(unfinished) > 0 {
		logger.Info("recovering unfinished jobs", "count", len(unfinished))
	}

	// The worker has its own context: it keeps running while HTTP requests
	// drain on shutdown, and stops after them. Deferred calls run in reverse
	// order, so it also stops before the database is closed.
	workerCtx, stopWorker := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() { jobs.Run(workerCtx) })
	defer func() {
		stopWorker()
		wg.Wait()
		logger.Info("worker stopped")
	}()

	srv := &http.Server{
		Addr: cfg.addr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Logger:      logger,
			CORSOrigins: cfg.corsOrigins,
			Printers:    printers,
			Jobs:        db,
			Documents:   documents,
			Worker:      jobs,
			Auth:        sessions,

			CookieSecure: cfg.cookieSecure,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// ListenAndServe blocks, so it runs in its own goroutine while this one
	// waits for either a server error or a shutdown signal.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.addr, "cors_origins", cfg.corsOrigins)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown stops accepting connections and waits for active requests to finish.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.Info("api stopped")
	return nil
}
