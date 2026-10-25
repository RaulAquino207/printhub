# HTTP server and graceful shutdown in Go

How the PrintHub API starts, routes requests and stops without dropping in-flight requests.

## 1. The problem

An HTTP API needs to listen on a port, route requests to handlers, apply cross-cutting behavior (logging, panic recovery) and stop cleanly. Stopping matters: Docker, systemd and the hot-reload tool (air) all stop the process with a signal. Killing it abruptly can cut off requests halfway — later, that could mean a half-received PDF upload.

## 2. The theory

**`http.Handler`** is the core abstraction of Go's `net/http`:

```go
type Handler interface {
    ServeHTTP(http.ResponseWriter, *http.Request)
}
```

Anything with that method can handle requests. `http.HandlerFunc` adapts a plain function to the interface, so a handler is usually just a function. There are no controller classes and no decorators: a router maps method + path to a handler.

**Middleware** is a function that wraps a handler and returns another handler: `func(http.Handler) http.Handler`. Each middleware runs code before and/or after calling `next.ServeHTTP`. This is the same idea as NestJS interceptors/middleware, expressed as plain function composition.

**`context.Context`** carries cancellation and deadlines through a call chain. `signal.NotifyContext` returns a context that is cancelled when the process receives a signal (SIGINT on Ctrl+C, SIGTERM from Docker/systemd).

**Goroutines and channels**: `srv.ListenAndServe()` blocks until the server stops, so it runs in a separate goroutine (a lightweight thread managed by the Go runtime). A channel carries its result back, and `select` waits for whichever happens first: a server error or a shutdown signal.

**`srv.Shutdown(ctx)`** stops accepting new connections and waits for active requests to finish, up to the context's deadline.

## 3. The chosen solution

- **Chi** as router: it is built on `net/http` types, so handlers and middleware stay standard and portable.
- `main` only builds the logger and calls `run`, which returns an `error`. This keeps a single exit path (`os.Exit(1)` on error) and makes `defer` statements run, because `os.Exit` skips deferred calls.
- Configuration comes from environment variables with safe defaults. The default address is `localhost:8080`, so the API is not reachable from the local network during development.
- Server timeouts: `ReadHeaderTimeout` protects against clients that open connections and send headers slowly; `IdleTimeout` closes idle keep-alive connections. A full `ReadTimeout` is not set yet because large uploads will need a deliberate value.
- Shutdown waits up to 10 seconds for in-flight requests.

## 4. Example from our project

The middleware chain in `apps/api/internal/httpapi/router.go`:

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)      // assigns an ID to every request
r.Use(logRequests(logger))       // logs method, path, status, duration
r.Use(middleware.Recoverer)      // turns a panic into a 500 instead of crashing
r.Get("/healthz", handleHealth)
```

Order matters: `Recoverer` sits inside `logRequests`, so a recovered panic is still logged with status 500.

The wait-for-signal logic in `apps/api/cmd/printhub-api/main.go`:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

serveErr := make(chan error, 1)
go func() { serveErr <- srv.ListenAndServe() }()

select {
case err := <-serveErr: // the server could not start (e.g. port in use)
    return fmt.Errorf("listen: %w", err)
case <-ctx.Done(): // Ctrl+C or SIGTERM
}

shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
defer cancel()
return srv.Shutdown(shutdownCtx)
```

The channel has a buffer of 1 so the goroutine can always send its result and exit, even if nobody is receiving anymore.

`fmt.Errorf("listen: %w", err)` wraps the original error with context; `%w` keeps it inspectable with `errors.Is` / `errors.As`. This replaces the exception + exception filter pattern from NestJS: errors are values returned and handled explicitly.

The handler response uses a struct with a **struct tag** to control the JSON field name — the closest thing to a DTO:

```go
type healthResponse struct {
    Status string `json:"status"`
}
```

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| Standard library `http.ServeMux` only | Since Go 1.22 it supports methods and path parameters; enough for small APIs. Chi adds route groups, a middleware stack and ready-made middleware, while staying compatible with `net/http` |
| Gin / Echo / Fiber | Popular, but use their own context types instead of `http.Handler`, so code is less portable. Fiber is not built on `net/http` at all |
| `log` package or a third-party logger (zap, zerolog) | `log/slog` is in the standard library, structured, and sufficient here |
| No graceful shutdown | Simpler, but in-flight requests are cut when the process stops |

## 6. How to validate

```bash
moon run api:test                  # httptest-based handler test
moon run api:dev                   # starts with hot reload on localhost:8080
curl -i localhost:8080/healthz     # 200, {"status":"ok"}
curl -i localhost:8080/nope        # 404
# Edit any .go file: air logs "has changed", then "shutting down" → "api listening"
# Ctrl+C: logs "shutting down" and "api stopped"
```
