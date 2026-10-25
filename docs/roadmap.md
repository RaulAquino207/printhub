# Roadmap

Small milestones, each reviewed and tested locally before it is committed. Workflow: Learn → Plan → Approve → Implement → Test locally → Review → Approve commit.

Commit messages below are suggestions; the actual history is in `git log`.

**Progress:** milestones 0–9 done (2026-10-25). Next: milestone 10.

## Phase 1 — Local development

| # | Milestone | Main learning | Demo / acceptance | Suggested commits |
|---|---|---|---|---|
| 0 | Repository bootstrap: base files, docs, ADRs | ADRs, Conventional Commits | Docs reviewed | `chore: initialize repository`, `docs: add architecture overview, roadmap and hardware notes`, `docs: add initial architecture decision records` |
| 1 | proto + moon workspace | proto vs moon vs Go modules vs pnpm; toolchains | `moon setup` and `moon query projects` work | `chore: configure proto and moon workspace` |
| 2 | API skeleton: `/healthz`, config from env, structured logs, graceful shutdown, hot reload | packages, `main`, `http.Handler`, Chi, `context`, `signal.NotifyContext`; moon tasks and cache | `moon run api:dev`; `curl localhost:8080/healthz` | `chore(api): initialize go module`, `feat(api): add health endpoint` |
| 3 | Web skeleton: shadcn preset (Base UI), design tokens, pt-BR app shell, API status via CORS | App Router, layouts, Server vs Client Components, theme tokens, CORS preflight | `moon run :dev`; `localhost:3000` shows API status | `chore(web): scaffold next app with shadcn preset`, `feat(web): add app shell`, `feat(api): add cors middleware` |
| 4 | Printer domain + mock provider, `GET /printers`, list in the web | structs, methods, small interfaces, value vs pointer receivers | Simulated printers visible in the browser | `feat(api): add printer domain`, `feat(api): add mock printing provider`, `feat(web): list printers` |
| 5 | Print options validation + job state machine (pure logic) | custom errors, `errors.Is/As`, table-driven tests | `moon run api:test` covers every transition | `feat(api): add print job lifecycle`, `test(api): cover job state transitions` |
| 6 | SQLite persistence + migrations | `database/sql`, `embed`, `sql.ErrNoRows`, `defer` | Data survives an API restart | `feat(api): persist printers and jobs in sqlite` |
| 7 | PDF upload + `POST /jobs` | `multipart`, `http.MaxBytesReader`, `io.Copy`, streams | Valid PDF accepted; oversized / non-PDF rejected | `feat(api): support pdf uploads` |
| 8 | Worker, cancellation, restart reconciliation | goroutines, channels, `select`, `sync`, race detector | Simulated jobs progress, fail, cancel, and resume after restart | `feat(api): process jobs in background`, `feat(api): support job cancellation` |
| 9 | "Nova impressão" screen | forms, upload states, accessibility | Full simulated flow at 390px and 1440px | `feat(web): add new print job screen` |
| 10 | "Fila de impressão" screen | data fetching, polling, empty and error states | Queue updates while jobs progress | `feat(web): add print queue` |
| 11 | Session authentication + CSRF protection | middleware, cookies, password hashing, credentialed CORS | Login required; cross-origin requests from unknown origins rejected | `feat(api): add session authentication`, `feat(web): add login` |
| 12 | CUPS provider against a local containerized CUPS with a virtual queue | IPP, timeouts, integration build tag | Job reaches the virtual queue (not physical printing) | `feat(api): add cups printing provider` |
| 13 | Raspberry Pi bridge and printer validation | CUPS, PPDs, filters, USB | Real test pages per printer, recorded in `docs/hardware.md` | `docs: record hardware validation` |

## Phase 2 — Production (only after explicit approval)

| # | Milestone | Notes |
|---|---|---|
| 14 | Two Docker images (web, api) | Multi-stage builds; Next.js `standalone` output |
| 15 | GitHub remote + GitHub Actions with `moon ci` | Requires approval to create the remote |
| 16 | Coolify services, Traefik routes, TLS, CoreDNS | Resolve the conflict between Coolify's proxy and the existing Traefik first; `*.homelab` needs an internal CA |

## Future (not planned yet)

PDF preview, richer history, retries, more document formats, scanning, Home Assistant power control, per-user permissions, a generated API contract package shared by web and api.
