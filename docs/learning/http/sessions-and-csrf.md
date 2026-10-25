# Sessions, cookies and CSRF

How PrintHub signs users in across two origins, keeps sessions safe, and stops other sites from acting on a signed-in user's behalf.

## 1. The problem

Tailscale limits who can *reach* PrintHub, but every device on the tailnet (and anything running on it) could still print, cancel jobs or read document names. The application needs its own authentication — and because the web (`printhub.homelab`) and the API (`api.printhub.homelab`) are different origins ([ADR 0003](../../adr/0003-subdomain-routing-and-cors.md)), the session must work across them without opening the door to cross-site attacks.

## 2. The theory

- **Password storage**: never store passwords, store a slow, salted hash. **bcrypt** is deliberately expensive (cost 10 ≈ tens of ms), which makes guessing millions of passwords impractical. It only uses the first 72 bytes, so longer passwords are refused instead of silently truncated.
- **Sessions**: after login the server issues a random **token**; the browser sends it back on every request. Tokens are long and random, so a fast hash (SHA-256) is enough to store them — and storing only the hash means a leaked database does not leak usable sessions.
- **Cookie attributes**:
  - `HttpOnly` — JavaScript cannot read it, so an XSS bug cannot steal the session;
  - `Secure` — sent over HTTPS only (browsers treat `http://localhost` as secure, so development works over HTTP);
  - `SameSite=Lax` — not sent on cross-*site* subrequests (forms, fetches, images from another site), only on top-level navigations;
  - no `Domain` — *host-only*: sent only to the exact host that set it;
  - `Max-Age` — lifetime; a negative value deletes it.
- **Credentialed CORS**: a cross-origin `fetch` sends cookies only with `credentials: "include"`, and the browser exposes the response only if the server answers with the exact origin and `Access-Control-Allow-Credentials: true`.
- **CSRF** (cross-site request forgery): a malicious page makes the victim's browser send a request to PrintHub; the browser attaches the cookie automatically. Defenses used here:
  1. `SameSite=Lax` — the cookie is not attached to cross-site subrequests at all;
  2. **Origin check** — browsers add an `Origin` header to cross-origin and POST requests, and pages cannot forge it;
  3. **custom header** (`X-Requested-With`) — a page on another origin can only add custom headers after a CORS preflight, which our allowlist refuses.
- **Timing attacks on login**: answering faster for unknown usernames reveals which ones exist. Comparing against a dummy hash equalizes the time.
- **Open redirects**: a `?next=` parameter must only accept local paths, or a crafted link could send users to another site after signing in.

## 3. The chosen solution

**API** (`internal/auth`, `internal/httpapi/auth.go`):

| Piece | Choice |
|---|---|
| Users | `users` table; usernames `[a-z0-9._-]{3,32}`, case-insensitive; passwords 8–72 bytes, bcrypt cost 10 |
| Creating users | `printhub-api user add <username>` on the server (no sign-up screen); password typed without echo or piped through stdin |
| Sessions | `crypto/rand.Text()` token (~130 bits); `sessions` table stores its SHA-256; 30 days; expired rows deleted at startup |
| Cookie | `printhub_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`, host-only, `Secure` when `PRINTHUB_COOKIE_SECURE=true` |
| Endpoints | `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me` |
| Protection | every `/api/v1` route except login requires a session (`401 unauthenticated`); `/healthz` stays public |
| CSRF | every non-GET `/api/v1` request needs an allowed `Origin` **and** `X-Requested-With` (`403 csrf_rejected`) |
| Attribution | each job stores the username that created it |

**Web** (`components/session-provider.tsx`, `app/entrar`):

- The cookie is `HttpOnly` and belongs to the API host, so the web cannot read it; it asks `GET /api/v1/auth/me` instead.
- `SessionProvider` holds the session state and redirects signed-out visitors to `/entrar?next=<path>`; `SessionGate` renders pages only for signed-in users. This is navigation convenience — the API enforces access on every request.
- Any `401` from the API (an expired session, a logout in another tab) fires an event that signs the app out.
- `apiFetch` adds `X-Requested-With` to every non-GET request; the upload's `XMLHttpRequest` does the same.

## 4. Example from our project

The middleware chain (`router.go`):

```go
r.Route("/api/v1", func(r chi.Router) {
	r.Use(checkCSRF(deps.CORSOrigins))   // all unsafe methods, login included
	r.Post("/auth/login", s.login)
	r.Group(func(r chi.Router) {
		r.Use(s.requireUser)             // 401 without a valid session
		r.Get("/jobs", s.listJobs)
		// ...
	})
})
```

Passing the user to handlers through the request context, with an unexported key type that no other package can collide with:

```go
type userKey struct{}
next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
```

The cookie (`auth.go`) — note the absent `Domain`:

```go
&http.Cookie{Name: "printhub_session", Value: token, Path: "/", MaxAge: maxAge,
	HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cookieSecure}
```

Using the API from a terminal now needs a session and, for changes, the CSRF headers:

```bash
H=(-H 'Origin: http://localhost:3000' -H 'X-Requested-With: PrintHub')
curl -c jar.txt "${H[@]}" -X POST localhost:8080/api/v1/auth/login -d '{"username":"raul","password":"…"}'
curl -b jar.txt localhost:8080/api/v1/jobs
curl -b jar.txt "${H[@]}" -X POST localhost:8080/api/v1/jobs/<id>/cancel
```

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| JWT in `localStorage` | Stateless, but readable by any XSS and impossible to revoke before expiry |
| Cookie with `Domain=printhub.homelab` | Enables authenticated SSR, but the cookie reaches more hosts; not needed because all data is fetched client-side |
| Synchronizer CSRF token | Strongest classic defense; redundant here given SameSite + Origin + custom header, and more state to manage |
| Tailscale identity (whois) instead of passwords | No passwords at all, but needs the `tailscaled` socket inside the API container, which grants broad privileges |
| argon2id instead of bcrypt | Memory-hard and the modern recommendation; bcrypt is simpler and adequate here |
| `SameSite=Strict` | Also blocks the cookie on top-level navigations from other sites; no benefit for an API that is never navigated to |

Known limitations: no login rate limiting (bcrypt's cost and the private network are the mitigation for now) and no user management beyond `user add`.

## 6. How to validate

```bash
moon run api:test   # auth service with real SQLite, cookie flags, 401/403 cases, logout, CLI
cd apps/api && go run ./cmd/printhub-api user add raul    # create your account
moon run api:dev web:dev
```

- `http://localhost:3000/fila` → redirected to `/entrar?next=%2Ffila`; signing in returns to the queue.
- DevTools → Application → Cookies → `http://localhost:8080`: `printhub_session` is `HttpOnly`, `SameSite=Lax`.
- `sqlite3 apps/api/data/printhub.db 'SELECT token_hash FROM sessions'` shows 64-character hashes, never the cookie value.
- `curl -X POST localhost:8080/api/v1/auth/login -d '{}'` (no Origin, no header) → `403 csrf_rejected`.
