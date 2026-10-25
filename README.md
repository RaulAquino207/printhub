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

## Documentation

- [Architecture](docs/architecture.md)
- [Roadmap](docs/roadmap.md)
- [Hardware](docs/hardware.md)
- [Architecture Decision Records](docs/adr/)
- [Design system](DESIGN_SYSTEM_MINIMAL_MODERN.md)
- [Agent instructions](AGENTS.md)
