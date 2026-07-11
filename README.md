# Keel

Scaffold production-grade fintech backends.

Keel generates the structure and the system-design patterns a payments backend
needs from day one — idempotency, a double-entry ledger, rate limiting, webhook
verification, security hardening — as idiomatic, editable code you fully own.
No runtime dependency, no framework, no lock-in. When generation finishes, Keel
is gone and you have a plain Go or TypeScript service.

> **Status:** early development. The architecture is settled; the engine is
> being built in the open. See [`docs/architecture.md`](docs/architecture.md).

## Why

Payments backends re-solve the same problems, and the failure modes are
expensive: money stored as floats, ledgers that don't balance, retries that
double-charge, webhooks that trust unsigned payloads. The correct patterns are
well understood by people who've shipped payments and unknown to everyone else.
Keel encodes them so a team starts from a correct foundation instead of
reconstructing one under a deadline.

## How it works

Keel is a single Go binary. You pick a language and a set of modules; it
resolves their dependencies and generates a coherent project.

```sh
keel init myservice --lang go --modules idempotency,ratelimit,ledger
```

Each module is a self-contained pattern with per-language implementations.
Modules declare their dependencies and conflicts in a manifest, so the generated
project is composed, not copied — the pieces fit together and compile.

## Design principles

- **Generated code is owned code.** Idiomatic, dependency-light, maintainable as
  if you wrote it.
- **Correctness in the money path is not optional.** Integer minor units,
  append-only ledger, idempotency keys, pessimistic locking — defaults, not
  features to remember.
- **Composition over configuration.** Small modules with explicit dependencies.
- **Language parity is a contract.** A module means the same thing in every
  language it supports.

## Contributing

Keel is MIT licensed. Module contributions are the highest-leverage way to help
once the engine lands; the manifest format and authoring guide are documented in
[`docs/`](docs/).

## License

[MIT](LICENSE)
