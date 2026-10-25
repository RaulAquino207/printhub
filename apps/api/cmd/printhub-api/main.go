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

	"github.com/RaulAquino207/printhub/apps/api/internal/document"
	"github.com/RaulAquino207/printhub/apps/api/internal/httpapi"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
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

	return config{
		addr:           envOr("PRINTHUB_API_ADDR", "localhost:8080"),
		corsOrigins:    origins,
		dataDir:        envOr("PRINTHUB_DATA_DIR", "data"),
		maxUploadBytes: int64(maxUploadMB) << 20,
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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

	documents, err := document.NewStorage(filepath.Join(cfg.dataDir, "documents"), cfg.maxUploadBytes)
	if err != nil {
		return err
	}

	// Only simulated printers exist until the CUPS provider is implemented.
	printerConfigs, provider := mock.Demo()
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
