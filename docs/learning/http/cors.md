# CORS in practice

How the PrintHub web app (one origin) is allowed to call the API (another origin) with credentials — and what CORS does and does not protect.

## 1. The problem

The web runs on `http://localhost:3000` and the API on `http://localhost:8080` (in production, `printhub.homelab` and `api.printhub.homelab`, see [ADR 0003](../../adr/0003-subdomain-routing-and-cors.md)). Browsers apply the **same-origin policy**: a page may *send* requests anywhere, but by default it may not *read* responses from a different origin. Without extra work, the web app could not read anything the API returns.

## 2. The theory

- **Origin** = scheme + host + port. `localhost:3000` and `localhost:8080` are different origins.
- **CORS** (Cross-Origin Resource Sharing) lets a server tell the browser which other origins may read its responses, through response headers:
  - `Access-Control-Allow-Origin` — the origin allowed to read the response.
  - `Access-Control-Allow-Credentials: true` — the response may be read even when the request carried cookies. When present, the allowed origin must be **exact**; `*` is refused by browsers.
  - `Vary: Origin` — the response differs per origin, so caches must not mix them up.
- **Simple requests** (e.g. `GET` without custom headers) are sent directly; the browser checks the CORS headers on the response.
- **Preflight**: for other requests (`POST` with JSON, custom headers such as `X-Requested-With`, …) the browser first sends an `OPTIONS` request with `Access-Control-Request-Method` / `-Headers`. The real request is only sent if the preflight response allows it.
- **CORS is not access control.** A disallowed request still reaches the server and is processed; the browser only hides the response from the page. Non-browser clients (`curl`, scripts) ignore CORS entirely. Authentication and CSRF protection are separate layers.

## 3. The chosen solution

A small handwritten middleware in the API (`apps/api/internal/httpapi/cors.go`) instead of a library — it is short, security-relevant and worth understanding:

- Origins come from `PRINTHUB_CORS_ALLOWED_ORIGINS` (default `http://localhost:3000`); the API refuses to start with `*`.
- Allowed origin → echo the exact origin + `Access-Control-Allow-Credentials: true`.
- Preflight from an allowed origin → `204` with the allowed methods (`GET, POST`) and headers (`Content-Type, X-Requested-With`), cached by the browser for 10 minutes (`Access-Control-Max-Age: 600`).
- Any other origin → no CORS headers at all, so the browser blocks the page from reading the response.

On the web side, every call goes through `apiFetch` (`apps/web/lib/api.ts`), which sends `credentials: "include"` so session cookies will travel once authentication exists.

## 4. Example from our project

```bash
# Allowed origin: the browser may read the response
curl -si -H 'Origin: http://localhost:3000' localhost:8080/healthz
# Access-Control-Allow-Origin: http://localhost:3000
# Access-Control-Allow-Credentials: true
# Vary: Origin

# Unknown origin: no CORS headers → the browser hides the response
curl -si -H 'Origin: http://evil.example' localhost:8080/healthz
# HTTP/1.1 200 OK   ← the API still answered!
# Vary: Origin

# Preflight
curl -si -X OPTIONS -H 'Origin: http://localhost:3000' \
  -H 'Access-Control-Request-Method: POST' localhost:8080/healthz
# HTTP/1.1 204 No Content
# Access-Control-Allow-Methods: GET, POST
# Access-Control-Allow-Headers: Content-Type, X-Requested-With
```

Experiment done during milestone 3: running the API with `PRINTHUB_CORS_ALLOWED_ORIGINS=http://other.test` made the header show "API indisponível", while the API log showed the `/healthz` requests answered with `200`. That is the whole point of the theory section: the request arrived, only the browser refused to hand the response to the page.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| Same-origin routing (`/api` on the web's host) | No CORS at all; rejected on purpose to learn cross-origin handling (ADR 0003) |
| `github.com/go-chi/cors` or `rs/cors` | Battle-tested and feature-complete; less to learn, one more dependency |
| `Access-Control-Allow-Origin: *` | Fine for public, cookie-less APIs; incompatible with credentials and unacceptable here |
| Reflecting any `Origin` back | Looks like an allowlist but allows every site to read authenticated responses — a classic vulnerability |

## 6. How to validate

```bash
moon run api:test                  # TestCORS covers allowed, unknown, preflight and no-origin cases
moon run api:dev web:dev           # header shows "API online"
# Stop the API → after up to 10 s the header shows "API indisponível"
# DevTools → Network → /healthz → Response headers show Access-Control-Allow-*
```
