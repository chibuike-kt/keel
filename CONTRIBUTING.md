# Contributing to Keel

Thanks for considering a contribution. Keel aims to generate code that senior
engineers trust, so the bar for what goes in is deliberately high. This document
covers how the project is built and how to get a change merged.

## Ground rules

- Discuss non-trivial changes in an issue first. A new module especially should
  start as a [module proposal](.github/ISSUE_TEMPLATE/module_proposal.yml).
- Keep changes focused. One concern per branch, one logical change per commit.
- Do not add AI-tool attribution — no `Co-Authored-By` trailers, no generated
  "authored by" notes in commits, code, or docs. Authorship stays human.

## Branching model

Keel uses trunk-based development (GitHub Flow). `main` is always releasable and
protected; all work happens on short-lived branches off `main` and returns
through a pull request.

Branch names carry a type prefix and a short kebab-case description:

```
feat/manifest-loader
fix/resolver-cycle-detection
docs/module-authoring
refactor/renderer-writes
test/idempotency-replay
chore/ci-cache
```

## Commits

Commits follow [Conventional Commits](https://www.conventionalcommits.org).

```
<type>(<optional scope>): <imperative summary>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`.

```
feat(resolver): detect dependency cycles before rendering
fix(renderer): roll back partial writes on template error
docs: document the module manifest schema
```

A commit should build and pass its own tests in isolation. Subject line in the
imperative mood, no trailing period, under ~72 characters. Explain the *why* in
the body when it isn't obvious.

## Pull requests

1. Branch off the latest `main`.
2. Make atomic commits as you go.
3. Open a PR into `main`. Fill in the template.
4. CI must pass — lint, test, and build are required checks.
5. Self-review the diff as if it were someone else's. Then merge.

Feature branches are integrated with **merge commits** (`--no-ff`), which keep
each atomic commit and preserve the branch boundary in history. Do not squash —
it collapses the granular history the project intentionally keeps.

## Local development

```sh
# Build the CLI
go build ./...

# Run the full test suite with the race detector
go test -race ./...

# Lint (matches CI; requires golangci-lint v2)
golangci-lint run
```

Install golangci-lint from https://golangci-lint.run/docs/welcome/install. The
repository pins its configuration to v2 in [`.golangci.yml`](.golangci.yml).

## Standards for generated code

Generated output is the product. Hold it to production standards:

- Idiomatic per language. Go reads like Go; TypeScript reads like TypeScript.
- Comment the *why*, never the *what*. Delete comments that restate the code.
- Money is integer minor units. Ledgers are append-only and balance. Replayable
  writes carry idempotency keys.
- Every module ships tests, and a freshly generated project compiles with its
  tests passing.

## Adding a module

A module is a self-contained pattern with per-language implementations, declared
in a `module.yaml` manifest. The manifest owns dependency and ordering logic;
templates stay free of it. See [`docs/architecture.md`](docs/architecture.md)
for the schema and the resolver's guarantees. A module is complete only when it
has parity across the languages it claims to support.

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT license](LICENSE).
