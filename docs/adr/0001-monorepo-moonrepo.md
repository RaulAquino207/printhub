# 0001 — Monorepo managed with Moonrepo v2

- **Status:** Accepted
- **Date:** 2026-10-25

## Context

PrintHub has a Go API (`apps/api`) and a Next.js web app (`apps/web`) in one repository. Each ecosystem has its own native tooling: Go modules for Go dependencies and builds, pnpm for JavaScript dependencies, and Next.js for its own build cache.

Two options were compared:

- **Simple monorepo** — native tooling plus a Makefile. Lowest configuration, but nothing coordinates tasks, caching or "what changed" across projects.
- **Moonrepo v2** — a task runner and project graph for polyglot repos. In v2, toolchains are WASM plugins; Go and Node (npm / pnpm / yarn) are fully supported.

Turborepo and Nx were discarded: both are JavaScript-centric and treat Go as a second-class citizen (scripts or community plugins).

With only two projects and no shared packages, Moonrepo's graph and caching bring limited *technical* gain today. However, PrintHub is a learning project, and monorepo orchestration is an explicit learning goal.

## Decision

Use **Moonrepo v2 from day one**, with **proto** to pin tool versions.

Responsibilities:

| Tool | Owns |
|---|---|
| proto (`.prototools`) | Versions of moon, Go, Node and pnpm |
| moon (`.moon/`, `moon.yml`) | Project graph, task definitions, task caching, affected detection, CI orchestration |
| Go modules (`go.mod`) | Go dependencies and builds |
| pnpm (`package.json`) | JavaScript dependencies |

Configuration rules:

- Declare tasks explicitly in each project's `moon.yml`, with `inputs` and `outputs`, so caching is visible and understandable.
- Toolchain versions are inherited from `.prototools` (the moon v2 default).
- No task inheritance (`.moon/tasks/`), remote cache or Docker integration until there is a concrete need.

## Consequences

- One entry point for everyday tasks (`moon run :dev`, `moon run :test`), and later `moon ci` for affected-only CI.
- Extra configuration to learn and maintain; contributors need proto/moon installed.
- moon must not become a substitute for Go modules or pnpm; it only orchestrates them.
- Moon's Docker helpers (`moon docker scaffold/prune`) are documented mostly for Node; their value for the Go image is evaluated in the Docker milestone, not assumed.
- Moon's project graph becomes more valuable if a shared package appears (e.g. an API contract generated from the Go API and consumed by the web).

## References

- https://moonrepo.dev/docs
- https://moonrepo.dev/docs/config/toolchain
- https://moonrepo.dev/blog/moon-v2.0
