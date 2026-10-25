# 0002 — Separate deployments for the web and the API

- **Status:** Accepted
- **Date:** 2026-10-25

## Context

The web (Next.js) and the API (Go) live in the same monorepo. Two production topologies were considered:

- **Single artifact** — build Next.js as a static export and serve it from the Go binary with `embed`. One image, one port, no cross-origin requests. It gives up Next.js server features (Server Components rendered at request time, route handlers, Server Actions).
- **Two artifacts** — the Next.js app and the Go API run as separate services with their own images and deployments.

PrintHub is also a learning project: Next.js server-side rendering, independent deployments, Docker and Coolify are explicit learning goals.

## Decision

Build and deploy the web and the API **independently**:

- One GitHub monorepo, two applications (`apps/web`, `apps/api`).
- Two Docker images (later milestone).
- Two Coolify services, deployable independently (later milestone).
- During the current phase, both run locally as independent processes (`localhost:3000`, `localhost:8080`).

The API owns business logic, validation, persistence, jobs and printing-provider communication. The web owns the user interface only.

## Consequences

- Each side can be built, released and rolled back on its own; moon's affected detection can skip untouched projects in CI.
- The browser talks to the API cross-origin, which requires CORS and careful cookie handling ([ADR 0003](0003-subdomain-routing-and-cors.md)).
- Two runtimes in production (Node.js and Go) instead of one, and two services to monitor.
- The API contract must stay stable between independently deployed versions; it is versioned under `/api/v1`.
- No production infrastructure is built until explicitly approved.
