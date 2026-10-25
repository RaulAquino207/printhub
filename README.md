# PrintHub

Self-hosted web application for managing document printing in a private HomeLab: upload a PDF, pick a printer, choose the options it supports, submit, and follow the job.

PrintHub is also a learning project (Go, Next.js, Moonrepo, CUPS/IPP, DevOps), so decisions are documented along with their reasoning.

## Status

Early development. Nothing is implemented yet; printing will be **simulated** until real hardware is validated. See [docs/roadmap.md](docs/roadmap.md).

## Stack

| Area | Choice |
|---|---|
| Monorepo | Moonrepo v2 + proto |
| API (`apps/api`) | Go, Chi, SQLite |
| Web (`apps/web`) | Next.js (App Router), TypeScript, Tailwind CSS, shadcn/ui (Base UI), Lucide |
| Printing | CUPS / IPP, via a Raspberry Pi Zero WH print bridge (planned) |

## Development setup

Tool versions are pinned in [`.prototools`](.prototools) and installed by [proto](https://moonrepo.dev/proto); tasks are orchestrated by [moon](https://moonrepo.dev/moon).

```bash
# 1. Install proto (once per machine)
curl -fsSL https://moonrepo.dev/install/proto.sh | bash

# 2. Install the pinned versions of proto, moon, Go, Node.js and pnpm
proto install

# 3. Prepare moon's toolchains
moon setup
```

How the pieces fit together: [docs/learning/infra/proto-and-moon.md](docs/learning/infra/proto-and-moon.md).

## Running locally

Create your account once (users are created on the server; there is no sign-up screen):

```bash
cd apps/api && go run ./cmd/printhub-api user add <username>   # asks for the password twice
```

Then:

```bash
moon run api:dev web:dev   # API on http://localhost:8080, web on http://localhost:3000 (both hot reload)
curl localhost:8080/healthz

moon run :test :vet :typecheck :lint :format :build   # every check of every project
```

| Variable | Default | Purpose |
|---|---|---|
| `PRINTHUB_API_ADDR` | `localhost:8080` | Address the API listens on |
| `PRINTHUB_DATA_DIR` | `data` (relative to `apps/api` when run by moon) | Directory of the SQLite database `printhub.db`; created with `0700` |
| `PRINTHUB_MAX_UPLOAD_MB` | `50` | Largest PDF accepted, in MiB |
| `PRINTHUB_PRINTING_PROVIDER` | `mock` | `mock` (simulated demo printers) or `cups` |
| `PRINTHUB_CUPS_URL` | — | CUPS server for the `cups` provider, e.g. `http://localhost:6631` |
| `PRINTHUB_CUPS_SIMULATED` | `false` | Marks every CUPS printer as simulated; set it for the development CUPS container |
| `PRINTHUB_COOKIE_SECURE` | `false` | Set to `true` whenever the API is served over HTTPS, so the session cookie is HTTPS-only |
| `NEXT_PUBLIC_API_URL` (web) | `http://localhost:8080` | API base URL used by the browser; inlined at build time |
| `NEXT_PUBLIC_MAX_UPLOAD_MB` (web) | `50` | Upload limit for client-side feedback; keep it equal to `PRINTHUB_MAX_UPLOAD_MB` |
| `PRINTHUB_CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated browser origins allowed to call the API with credentials; `*` is rejected ([ADR 0003](docs/adr/0003-subdomain-routing-and-cors.md)) |

### Printing through a real CUPS server (no printer needed)

```bash
moon run api:cups-up            # development CUPS with virtual queues on http://localhost:6631
moon run api:test-integration   # IPP integration tests against it
PRINTHUB_PRINTING_PROVIDER=cups PRINTHUB_CUPS_URL=http://localhost:6631 PRINTHUB_CUPS_SIMULATED=true \
  moon run api:dev web:dev
moon run api:cups-down
```

The virtual queues print to `/dev/null`. See [docs/learning/infra/ipp-and-cups.md](docs/learning/infra/ipp-and-cups.md).

## Documentation

- [Architecture](docs/architecture.md)
- [Roadmap](docs/roadmap.md)
- [Hardware](docs/hardware.md) and the [Raspberry Pi print bridge runbook](docs/runbooks/raspberry-pi-print-bridge.md)
- [Architecture Decision Records](docs/adr/)
- [Design system](DESIGN_SYSTEM_MINIMAL_MODERN.md)
- [Agent instructions](AGENTS.md)
