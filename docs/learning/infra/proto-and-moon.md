# proto and moon

How PrintHub pins tool versions and orchestrates tasks across a Go and a Node.js project.

## 1. The problem

The repository contains two ecosystems: a Go API and a Next.js web app. Without coordination:

- Each machine (and CI) may run a different Go, Node.js or pnpm version, so "works on my machine" bugs appear.
- Every project has its own commands (`go test`, `pnpm build`, …) and there is no single way to run "all tests" or "only what changed".
- Nothing knows which projects a change affects, so CI rebuilds everything.

## 2. The theory

Three separate concerns are often mixed up:

| Concern | Question it answers | Tool in PrintHub |
|---|---|---|
| Toolchain versions | Which Go / Node.js / pnpm binary runs here? | **proto** |
| Dependencies | Which libraries does this project use, at which versions? | **Go modules** (`go.mod`, `go.sum`), **pnpm** (`package.json`, lockfile) |
| Task orchestration | Which tasks exist, in what order do they run, can they be skipped? | **moon** |

**proto** is a multi-language version manager (similar in spirit to `nvm` + `.nvmrc`, but for many tools). It installs tools under `~/.proto` and puts *shims* on the `PATH`: when you run `go`, the shim reads the nearest `.prototools` and executes the pinned version.

**moon** is a task runner and build system for monorepos. It builds a **project graph** (which projects exist and how they depend on each other) and runs **tasks** defined per project. Before running a task it hashes the task's inputs; if the hash matches a previous run, it restores the cached outputs instead of running again. With VCS information it can also compute **affected** projects: only those touched by a change.

In moon v2, language support comes from **toolchain plugins** (WebAssembly). The Go plugin understands `go.mod` / `go.sum`; the JavaScript, Node.js and pnpm plugins understand `package.json` and lockfiles.

moon never replaces Go modules or pnpm. It calls them.

## 3. The chosen solution

Moonrepo v2 + proto from day one ([ADR 0001](../../adr/0001-monorepo-moonrepo.md)), with the smallest configuration that works:

- `.prototools` pins exact versions of proto, moon, Go, Node.js and pnpm.
- `.moon/workspace.yml` declares where projects live and which VCS is used.
- `.moon/toolchains.yml` enables the `go`, `javascript`, `node` and `pnpm` toolchains **without versions**: each one inherits its version from `.prototools` (`versionFromPrototools` defaults to `true`), so versions live in one place.
- Tasks will be declared explicitly in each project's `moon.yml`, with `inputs` and `outputs`, so caching stays visible.

## 4. Example from our project

```toml
# .prototools
proto = "0.63.2"
moon = "2.6.4"
go = "1.27.3"
node = "24.22.1"
pnpm = "11.28.8"
```

```yaml
# .moon/workspace.yml
projects:
  - "apps/*"        # every folder in apps/ with a moon.yml becomes a project
vcs:
  client: "git"
  defaultBranch: "main"   # base for "affected" comparisons
  provider: "github"
```

```yaml
# .moon/toolchains.yml
go: {}
javascript:
  packageManager: "pnpm"
node: {}
pnpm: {}
```

Lessons from the setup:

- Exact versions (not `~` / `^`) keep every environment reproducible; upgrades become explicit commits.
- Plugins have their own requirements: moon 2.6's Go plugin requires proto ≥ 0.60. Pinning `proto` itself in `.prototools` makes that requirement visible.
- pnpm 12 ships a placeholder `pnpm` script (no shebang) that its own installer replaces with a native binary. proto does not run that step, so shells can run it but other programs that spawn `pnpm` directly fail with `ENOEXEC` (seen when the shadcn CLI ran `pnpm install`). PrintHub pins pnpm 11 until this works.
- Node.js 24 (LTS) was chosen over 26 (just entering LTS) for compatibility with Next.js.

## 5. Alternatives and trade-offs

| Alternative | Why not (for now) |
|---|---|
| Native tooling + Makefile | Simplest, but no project graph, cache or affected detection — and monorepo orchestration is a learning goal |
| Turborepo | JavaScript-centric; Go only through `package.json` scripts |
| Nx | Go via a community plugin; heavier configuration |
| asdf / mise instead of proto | Comparable version managers, but proto integrates directly with moon's toolchains |

Costs of this choice: more configuration to learn, and every contributor (and CI) needs proto.

## 6. How to validate

```bash
proto install            # installs every pinned tool
go version               # go1.27.3
node --version           # v24.22.1
pnpm --version           # 11.28.8
moon --version           # moon 2.6.4
moon setup               # "Setup toolchains successfully!"
moon toolchain info go   # shows the Go plugin and its settings
moon query projects      # lists projects (empty until apps/ exists)
```
