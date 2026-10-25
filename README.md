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
| `NEXT_PUBLIC_API_URL` (web) | `http://localhost:8080` | API base URL used by the browser; inlined at build time |
| `NEXT_PUBLIC_MAX_UPLOAD_MB` (web) | `50` | Upload limit for client-side feedback; keep it equal to `PRINTHUB_MAX_UPLOAD_MB` |
| `PRINTHUB_CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated browser origins allowed to call the API with credentials; `*` is rejected ([ADR 0003](docs/adr/0003-subdomain-routing-and-cors.md)) |

## Documentation

- [Architecture](docs/architecture.md)
- [Roadmap](docs/roadmap.md)
- [Hardware](docs/hardware.md)
- [Architecture Decision Records](docs/adr/)
- [Design system](DESIGN_SYSTEM_MINIMAL_MODERN.md)
- [Agent instructions](AGENTS.md)
