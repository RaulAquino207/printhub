# Architecture

PrintHub lets authorized users on a private network upload a PDF, choose a printer and the options it supports, submit a print job and follow its status.

Status labels used in this document: **Implemented**, **Simulated**, **Planned**, **Not yet verified**.

## Overview

```text
                     local development                              production (Planned, later milestone)
Browser ──> http://localhost:3000  (web, next dev)        Browser ──Tailscale──> Traefik
   │                                                                 ├── printhub.homelab      ──> web container
   └──────> http://localhost:8080  (api, go)                         └── api.printhub.homelab  ──> api container
                │                                                                                   │
                └── MockPrintingProvider (Simulated)                     CUPS/IPP provider ──Wi-Fi──> Raspberry Pi Zero WH
                                                                                                     └── USB hub ──> M1132, CP1215
```

The web and the API are separate applications in one monorepo. They are developed, built and (later) deployed independently ([ADR 0002](adr/0002-separate-frontend-and-api-deployments.md)) and talk over HTTP across origins ([ADR 0003](adr/0003-subdomain-routing-and-cors.md)).

## Components

| Component | Location | Responsibility | Status |
|---|---|---|---|
| Web | `apps/web` | Next.js App Router UI in pt-BR: new print job, print queue | Implemented: shell, API status, new print job flow (upload, options, live status, cancel), print queue |
| API | `apps/api` | Go + Chi HTTP API: printers, capabilities, uploads, jobs, auth | Implemented: health, CORS, printers |
| Database | SQLite file owned by the API (`PRINTHUB_DATA_DIR/printhub.db`) | Jobs and their history; later users and sessions. Printers stay in configuration for now | Implemented: jobs |
| Authentication | `apps/api/internal/auth` + `/api/v1/auth/*` | Users (created with `printhub-api user add`), bcrypt passwords, 30-day sessions in an HttpOnly cookie, CSRF checks | Implemented |
| Job worker | `apps/api/internal/worker`, one goroutine in the API process | Moves jobs through their lifecycle, talks to the provider, recovers interrupted jobs, removes finished documents | Implemented |
| Mock provider | `apps/api/internal/printing/mock` | Simulated printers, capabilities, success, failure, offline, cancellation | Implemented (Simulated): jobs take 2 s queued + 5 s printing; "Laser com defeito" always fails |
| CUPS provider | `apps/api` | Real printing over IPP | Planned |
| Print bridge | Raspberry Pi Zero WH | CUPS + drivers next to the printers | Not yet verified — see [hardware.md](hardware.md) |
| Power controller | Separate component | Smart plugs via Home Assistant | Future, disabled |

## Boundaries

| API (source of truth) | Web (presentation) |
|---|---|
| Printers, capabilities, validation of print options | Layout, navigation, pt-BR copy |
| Upload limits, PDF content validation, storage outside any public path | Early client-side checks (size/type) for fast feedback only |
| Job state machine, worker, restart reconciliation | Polling and rendering of job state; loading, empty and error states |
| Users, sessions, authorization, CORS policy | Login form; sends credentialed requests |
| Printing provider and future power controller | No business rules |

The contract is versioned JSON under `/api/v1`. The web adapts the options it shows to the capabilities it receives and never branches on printer model names.

## Domain model

- **Printer** — `ID`, `Name`, `Model`, `Destination` (provider-specific queue or URI), `Status` (online / offline / unknown). Has many jobs.
- **Capabilities** — color, paper sizes, duplex, max copies, page ranges, orientations. Reported by the provider, not hardcoded.
- **Options** — what the user chose; validated against the printer's capabilities.
- **Job** — `ID`, printer snapshot (ID, name, model, simulated), document metadata, options, status, provider job ID, failure reason, history of status transitions.

## Job lifecycle

Implemented in `apps/api/internal/job` (pure logic, no persistence yet). Status values are lowercase in code and JSON.

```text
queued → processing → submitted → printing → completed
                          └──────────────────→ completed   (short jobs may finish unobserved)
any non-terminal status ──→ failed
queued, submitted, printing ──cancel──→ cancelled
```

| Status | Meaning |
|---|---|
| `queued` | Accepted by PrintHub, waiting for the worker |
| `processing` | The worker is validating the document and sending it to the provider. **Cannot be cancelled**: the outcome is submitted or failed |
| `submitted` | The provider accepted the job and returned its own job ID |
| `printing` | The provider reports the job as being processed |
| `completed` | The provider reports the job as finished. **This does not prove that paper came out**; the UI says "Enviado à impressora" |
| `failed` | Terminal, with a failure reason |
| `cancelled` | Terminal |

- Every status change goes through one method that validates first; a rejected change returns `*job.TransitionError` (matches `job.ErrInvalidTransition`) and leaves the job untouched.
- Each job keeps a history of `(status, time)` entries; creation and last update times are derived from it.
- Jobs persist in SQLite with a snapshot of their printer, saved after every transition. The worker's first pass after a restart recovers unfinished jobs: `queued` continues, `processing` → `failed: interrupted` (the provider may or may not have received it, and printing twice is worse), `submitted`/`printing` → ask the provider by provider job ID (`failed: provider_lost_job` if it no longer knows the job). Details: [sqlite-persistence.md](learning/go/sqlite-persistence.md), [goroutines-and-the-worker.md](learning/go/goroutines-and-the-worker.md).
- Failure reasons are stable codes: `printer_unavailable`, `document_missing`, `submit_failed`, `interrupted`, `provider_lost_job`, `printer_error`.
- Documents are deleted once their job finishes; orphans are removed at startup.

## Print options

Implemented in `apps/api/internal/printer/options.go`: copies, color mode, paper size, orientation, duplex and page ranges (e.g. `1-3, 5`), validated against the printer's capabilities. Validation reports every invalid field with a stable reason code: `unsupported`, `out_of_range`, `invalid_format`, `capabilities_unknown`.

## API (initial)

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | Liveness — **Implemented** |
| GET | `/api/v1/printers` | Printers with status and capabilities — **Implemented** (Simulated data) |
| GET | `/api/v1/printers/{id}` | One printer — **Implemented** (Simulated data) |
| POST | `/api/v1/jobs` | Multipart: `printerId`, `options` (JSON), `file` (PDF) → `201` queued job, processed in the background — **Implemented** |
| GET | `/api/v1/jobs` | List jobs, newest first: `?status=queued,printing` (comma-separated) and `?limit=` (default 50, max 200) — **Implemented** |
| GET | `/api/v1/jobs/{id}` | Job details, status and history — **Implemented** |
| POST | `/api/v1/jobs/{id}/cancel` | Cancel a job → `200`, `404`, or `409 job_not_cancellable` — **Implemented** |

| POST | `/api/v1/auth/login` | JSON `{username, password}` → sets the session cookie — **Implemented** |
| POST | `/api/v1/auth/logout` | Ends the session and deletes the cookie — **Implemented** |
| GET | `/api/v1/auth/me` | The signed-in user — **Implemented** |

Every `/api/v1` route except login requires a session (`401 unauthenticated`), and every non-GET request needs an allowed `Origin` plus `X-Requested-With` (`403 csrf_rejected`). `/healthz` is public. Details: [sessions-and-csrf.md](learning/http/sessions-and-csrf.md). Job status is fetched by polling.

### Conventions

- JSON field names are camelCase; empty lists are encoded as `[]`, never `null`.
- Printers carry `"simulated": true` when served by the mock provider; the web shows a "Simulada" badge.
- Every error has the same shape; `code` is stable and mapped to pt-BR messages by the web:

```json
{ "error": { "code": "printer_not_found", "message": "printer not found" } }
```

Validation errors (`422`) also list every invalid field with a stable reason, e.g. `file`/`not_pdf`, `printerId`/`unavailable`, `colorMode`/`unsupported`:

```json
{ "error": { "code": "validation_failed", "message": "the request has invalid fields",
  "fields": [{ "field": "colorMode", "reason": "unsupported" }] } }
```

- Times are RFC 3339 in UTC.
- Uploaded documents are stored as `PRINTHUB_DATA_DIR/documents/<job id>.pdf` (mode `0600`), never under the client's filename; see [streams-and-uploads.md](learning/go/streams-and-uploads.md).

Example printer:

```json
{
  "id": "mono-laser",
  "name": "Laser P&B",
  "model": "Simulated monochrome laser",
  "status": "online",
  "simulated": true,
  "capabilities": {
    "colorModes": ["monochrome"],
    "paperSizes": ["A4", "Letter"],
    "orientations": ["portrait", "landscape"],
    "duplex": false,
    "pageRanges": true,
    "maxCopies": 99
  }
}
```

## Environments

| | Local (current phase) | Production (Planned) |
|---|---|---|
| Web origin | `http://localhost:3000` | `https://printhub.homelab` |
| API origin | `http://localhost:8080` | `https://api.printhub.homelab` |
| Orchestration | `moon run` with hot reload | Two Docker images, two Coolify services |
| Printing | Mock provider | CUPS provider → Raspberry Pi |
| Access | This machine | Tailscale + application authentication |

Production infrastructure (Docker images, CI, Coolify, Traefik, TLS, DNS) is implemented only after explicit approval.
