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
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/httpapi"
	"github.com/RaulAquino207/printhub/apps/api/internal/printer"
	"github.com/RaulAquino207/printhub/apps/api/internal/printing/mock"
)

// shutdownTimeout bounds how long in-flight requests may take to finish on shutdown.
const shutdownTimeout = 10 * time.Second

type config struct {
	// addr is the TCP address to listen on. The default binds to localhost
	// only, so the API is not reachable from the local network during development.
	addr string
	// corsOrigins are the browser origins allowed to call the API (ADR 0003).
	corsOrigins []string
}

func loadConfig() (config, error) {
	origins := splitList(envOr("PRINTHUB_CORS_ALLOWED_ORIGINS", "http://localhost:3000"))
	if slices.Contains(origins, "*") {
		return config{}, errors.New("PRINTHUB_CORS_ALLOWED_ORIGINS must list explicit origins, not *")
	}

	return config{
		addr:        envOr("PRINTHUB_API_ADDR", "localhost:8080"),
		corsOrigins: origins,
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

	// Only simulated printers exist until the CUPS provider is implemented.
	printerConfigs, provider := mock.Demo()
	printers := printer.NewRegistry(printerConfigs, provider)

	srv := &http.Server{
		Addr: cfg.addr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Logger:      logger,
			CORSOrigins: cfg.corsOrigins,
			Printers:    printers,
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
