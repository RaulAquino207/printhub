# 0003 — Subdomain routing, CORS and session cookies

- **Status:** Accepted (cookie scope to be finalized in the authentication milestone)
- **Date:** 2026-10-25

## Context

With separate deployments ([ADR 0002](0002-separate-frontend-and-api-deployments.md)), the browser must reach both the web and the API. Options compared:

| Option | Summary | Trade-off |
|---|---|---|
| A. Same origin, path routing | Traefik sends `/api/*` to the API and the rest to the web | No CORS; simplest cookies |
| B. Next.js as a BFF | Every API call is proxied through Next.js route handlers | API not exposed; uploads pass through two services |
| C. Separate subdomains | Web and API on their own hostnames | Requires CORS, credentialed requests and deliberate cookie and CSRF handling |

Option C was chosen deliberately: learning how CORS, credentialed requests, cookies and CSRF work is a goal of the project.

## Decision

| Environment | Web | API |
|---|---|---|
| Local (current) | `http://localhost:3000` | `http://localhost:8080` |
| Production (later) | `https://printhub.homelab` | `https://api.printhub.homelab` |

### CORS

- An explicit allowlist of origins from configuration (e.g. `CORS_ALLOWED_ORIGINS=http://localhost:3000`). **Never `*`.**
- `Access-Control-Allow-Credentials: true`, which browsers only accept together with an exact (non-wildcard) origin.
- Reflect the request `Origin` only if it is on the allowlist; send `Vary: Origin`.
- Allow only the methods and headers the API uses; answer preflight (`OPTIONS`) requests.
- CORS is a browser policy, not authorization. Every request is still authenticated by the API.

### Sessions and cookies

- Session cookie: `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` in production.
- Web requests to the API use `credentials: "include"`.
- Cookie scope is decided in the authentication milestone:
  - **Host-only on the API** (no `Domain` attribute) — narrowest scope, but Next.js Server Components never receive the session, so authenticated data is fetched client-side.
  - **`Domain=printhub.homelab`** — both the web server and the API receive the cookie, enabling authenticated server-side rendering, at the cost of a wider scope.

### Same-site vs same-origin

- `localhost:3000` and `localhost:8080` are **cross-origin** (different ports) but **same-site** (ports are ignored when computing the site).
- `.homelab` is not on the Public Suffix List, so browsers treat `homelab` as the public suffix and `printhub.homelab` as the site. `printhub.homelab` and `api.printhub.homelab` are therefore **same-site**, and `SameSite=Lax` cookies are sent on credentialed cross-origin requests between them.
- Other HomeLab services (e.g. `portainer.homelab`) are **different sites**.

### CSRF protection

Defense in depth for every state-changing request (`POST`, `PUT`, `PATCH`, `DELETE`):

1. `SameSite=Lax` session cookie.
2. Reject requests whose `Origin` header is missing or not on the allowlist.
3. Require a custom request header (e.g. `X-Requested-With`), which forces a CORS preflight that unknown origins fail.

A synchronizer CSRF token can be added later if needed.

### HTTPS

`Secure` cookies require HTTPS. Browsers treat `http://localhost` as a secure context, so local development works over HTTP. No public CA issues certificates for `*.homelab`; production needs an internal CA trusted by the users' devices. This is solved in the production milestone.

## Consequences

- Real exposure to CORS, preflight requests, credentialed fetches and cookie attributes, in local development as well as production.
- More configuration than option A: allowed origins and cookie settings differ per environment and must be driven by configuration.
- CORS misconfiguration fails closed in the browser (blocked requests), which is easy to notice during local testing.
- Production requires two DNS names, two Traefik routes and certificates for both hostnames.

## References

- https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
- https://developer.mozilla.org/en-US/docs/Web/HTTP/Cookies
- https://web.dev/articles/same-site-same-origin
- https://publicsuffix.org/
