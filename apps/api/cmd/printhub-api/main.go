// Command printhub-api runs the PrintHub HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/httpapi"
)

// shutdownTimeout bounds how long in-flight requests may take to finish on shutdown.
const shutdownTimeout = 10 * time.Second

type config struct {
	// addr is the TCP address to listen on. The default binds to localhost
	// only, so the API is not reachable from the local network during development.
	addr string
}

func loadConfig() config {
	return config{
		addr: envOr("PRINTHUB_API_ADDR", "localhost:8080"),
	}
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
	cfg := loadConfig()

	// ctx is cancelled on Ctrl+C (SIGINT) or SIGTERM (sent by Docker, air, systemd).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           httpapi.NewRouter(logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// ListenAndServe blocks, so it runs in its own goroutine while this one
	// waits for either a server error or a shutdown signal.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.addr)
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
