# Working in this repository

Keel scaffolds production-grade fintech backends. The code here is meant to be
read and trusted by senior engineers. Hold that bar.

## Version control — hard rules

- **Never run `git commit`, `git push`, or any command that writes to history.**
  Staging is fine for inspection; committing is the maintainer's job, always.
- **Never add a co-author trailer, `Co-Authored-By`, or any attribution to an AI
  tool** in commit messages, code comments, or documentation.
- Do not create, amend, rebase, or tag. Leave the history entirely to the human.

If a change is ready, say so and stop. The maintainer commits it.

## Commits are atomic (guidance for the maintainer)

One feature or fix per commit. Do not batch unrelated changes. A commit should
compile and pass its own tests in isolation. Message style: imperative subject,
present tense, no trailing period — e.g. `add idempotency middleware for go
target`.

## Code standards

- **Idiomatic per language.** Go reads like Go; TypeScript reads like TypeScript.
  No cross-language accents.
- **Comment sparingly.** Explain *why*, never *what*. No comment restating the
  code on the next line. No docstring on a self-evident function. If a comment
  would only narrate the obvious, delete it.
- **No AI tells.** No "Here's the…", no over-explained blocks, no defensive
  hedging in prose. Write like an engineer who knows the domain.
- **Errors are handled, not swallowed.** In the money path, an unhandled edge is
  a bug, not a TODO.
- **Money is integer minor units.** Never float. Ledgers are append-only and
  balance. Writes that can replay carry idempotency keys.

## Tests

Every module ships its own tests. A generated project must compile and its tests
must pass immediately after generation. No feature is "done" without a test that
would fail if the behaviour regressed.

## Scope discipline

Prefer small, composable modules with explicit manifest dependencies over large
templates with flags. If a change grows a module's responsibility, that's a
signal to split it, not to widen it.
