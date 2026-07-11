# Keel — Architecture

Keel scaffolds production-grade fintech backends. It generates the project
structure, wiring, and the hard-won system-design patterns a payments backend
needs on day one — idempotency, a double-entry ledger, rate limiting, webhook
verification, security hardening — so teams start from a correct foundation
instead of rebuilding one under deadline pressure.

## Problem

Every fintech backend re-solves the same problems, badly and repeatedly:

- Money stored as floats. Ledgers that don't balance. Retries that double-charge.
- Idempotency bolted on after the first duplicate-payment incident.
- Webhook handlers that trust unsigned payloads.
- Rate limiting copied from a blog post and never load-tested.

These aren't framework problems — Express, Gin, and FastAPI don't ship opinions
about money. They're systems problems, and the correct patterns are well known
to people who've shipped payments before and unknown to everyone else. Keel
encodes those patterns as generated, editable, idiomatic code.

## What Keel is not

- **Not a framework.** Keel generates code and then gets out of the way. There
  is no `keel` import in the output, no runtime dependency, no lock-in. Generated
  projects are plain Go or TypeScript you fully own.
- **Not a boilerplate dump.** Every generated file is coherent with the others.
  Modules compose; they don't collide.
- **Not a payment processor.** Keel wires the seams (webhook verification,
  provider adapters) but never moves money itself.

## Design principles

1. **Generated code is owned code.** Output is idiomatic, dependency-light, and
   readable. A senior engineer should be able to delete Keel from their mind the
   moment generation finishes and maintain the result as if they'd written it.
2. **Correctness over convenience in the money path.** Integer minor units,
   append-only ledger, pessimistic locking, idempotency keys — non-negotiable
   defaults, not opt-ins.
3. **Composition over configuration.** Small modules with explicit dependencies,
   resolved into one coherent project — not a monolithic template with feature
   flags.
4. **Language parity is a contract.** A module means the same thing in Go and
   TypeScript. Implementations differ; behaviour and guarantees do not.

## System overview

Keel is a single Go binary with four internal subsystems and an external module
store.

```
                    ┌──────────────────────────────┐
                    │           keel CLI            │
                    │  init · add · list · update   │
                    └───────────────┬──────────────┘
                                    │
        ┌───────────────┬───────────┴──────┬──────────────────┐
        ▼               ▼                  ▼                  ▼
   ┌─────────┐    ┌───────────┐      ┌───────────┐      ┌───────────┐
   │ prompt  │    │ registry  │      │ resolver  │      │ renderer  │
   │  (TUI)  │    │ (manifests)│     │  (deps)   │      │ (templates)│
   └─────────┘    └───────────┘      └───────────┘      └───────────┘
                        │                                     │
                        ▼                                     ▼
                  ┌───────────────────────────────────────────────┐
                  │              module store                     │
                  │  modules/<name>/module.yaml                   │
                  │  modules/<name>/templates/<lang>/...          │
                  └───────────────────────────────────────────────┘
```

- **prompt** — interactive selection when flags are omitted; drives the same
  code path as the non-interactive flags.
- **registry** — loads and validates module manifests. The source of truth for
  what a module is, what it needs, and which languages it supports.
- **resolver** — takes the selected modules, walks their `requires` graph,
  detects cycles and conflicts, and produces a topologically ordered build set.
- **renderer** — executes templates against the resolved context and writes the
  target directory transactionally (stage to temp, then atomically move).

## Module system

A module is a self-contained system-design pattern with per-language
implementations. It is the unit of composition, versioning, and contribution.

```
modules/idempotency/
├── module.yaml                     # manifest: identity, deps, per-language spec
└── templates/
    ├── go/
    │   ├── middleware/idempotency.go.tmpl
    │   └── middleware/idempotency_test.go.tmpl
    └── typescript/
        └── src/middleware/idempotency.ts.tmpl
```

### Manifest schema

```yaml
apiVersion: keel/v1
name: idempotency
version: 1.0.0
summary: Idempotency-key middleware that makes unsafe writes replay-safe.
tags: [reliability, payments]

requires: []            # module names this depends on
conflicts: []           # modules that cannot coexist with this one

languages:
  go:
    dependencies:
      - module: github.com/redis/go-redis/v9
        version: v9.5.1
    templates:
      - from: templates/go/middleware/idempotency.go.tmpl
        to: internal/middleware/idempotency.go
      - from: templates/go/middleware/idempotency_test.go.tmpl
        to: internal/middleware/idempotency_test.go
  typescript:
    dependencies:
      - package: ioredis
        version: ^5.4.1
    templates:
      - from: templates/typescript/src/middleware/idempotency.ts.tmpl
        to: src/middleware/idempotency.ts
```

The manifest is the contract. The resolver reasons over `requires`, `conflicts`,
and `languages`; the renderer consumes `templates` and `dependencies`. Templates
never encode dependency or ordering logic — that lives in the manifest, so the
build graph is inspectable without executing anything.

### Resolution

Given a requested set, the resolver:

1. Expands the transitive `requires` closure.
2. Fails on any `conflicts` intersection, with the conflicting pair named.
3. Fails on cycles, naming the cycle.
4. Fails on any module lacking an implementation for the target language.
5. Emits a deterministic, topologically sorted build plan.

Resolution is pure and side-effect free. It can be dry-run (`--plan`) to print
what would be written without touching disk.

### Rendering

The renderer builds one context — project name, module package path, selected
language, enabled module set — and executes each template against it. Writes are
transactional: everything renders to a temp tree, and only a fully successful
render is moved into place. A failed generation leaves no half-written project.

Dependency manifests (`go.mod`, `package.json`) are merged, not overwritten:
each module contributes its declared dependencies, deduplicated with version
conflicts surfaced as errors rather than silently resolved.

## Distribution

One artifact, several front doors:

- **GitHub Releases** — cross-compiled binaries (linux/darwin/windows ×
  amd64/arm64) as the source of truth.
- **npm wrapper** — `npm i -g keel-cli`; a `postinstall` script downloads the
  matching release binary. Gives fintech devs the workflow they expect without
  Keel being written in Node.
- **Homebrew tap** and **`curl | sh`** installer for non-npm users.

The binary embeds its bundled modules at build time, so a fresh install
scaffolds offline. A later remote registry (see roadmap) is additive and never
required for the core flow.

## Roadmap

**Phase 1 — Core mechanic, Go output only.**
CLI skeleton, manifest loader, resolver, renderer. One end-to-end path: `keel
init` produces a compiling Go service with idempotency, rate limiting, and
health endpoints. Prove the engine before broadening it.

**Phase 2 — The fintech module set (Go).**
Double-entry ledger, webhook verification with provider presets, security
hardening, structured logging, config validation, DB migrations, test harness.

**Phase 3 — TypeScript parity.**
Second language across every existing module. This is where the language-parity
contract gets tested for real.

**Phase 4 — Remote registry & community modules.**
Versioned module store, `keel add` against a remote index, third-party modules
with a review bar. shadcn-style: you own the code, the registry is a convenience.

## Repository layout

```
keel/
├── cmd/keel/            # entrypoint
├── internal/
│   ├── cli/             # command wiring
│   ├── registry/        # manifest load + validate
│   ├── resolver/        # dependency & conflict resolution
│   ├── renderer/        # template execution + transactional writes
│   └── prompt/          # interactive selection
├── modules/             # module manifests + templates (the product)
├── docs/
└── Makefile
```

`internal/` is the engine and stays small. `modules/` is where the domain
knowledge lives and where most ongoing work happens.
