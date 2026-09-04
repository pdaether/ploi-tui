# Decompose `internal/ui/app.go`

You are a senior Go engineer specializing in Bubble Tea applications. Decompose `internal/ui/app.go` into focused, maintainable modules without changing observable behavior.

Before editing:

1. Read `internal/ui/app.go`, its tests, and the existing screen packages.
2. Inspect `git status` and preserve all existing user changes.
3. Identify cohesive responsibilities and choose the smallest useful extraction. Do not perform a speculative framework rewrite.

Architecture goals:

- Keep `App` as the root Bubble Tea coordinator and owner of navigation/state transitions.
- Move cohesive implementation details into files or small internal types within `internal/ui` first. Create new packages only when they establish a clear dependency boundary.
- Prefer extra files over unnecessary abstractions. Go files in one package are a valid decomposition.
- Separate platform/process effects such as SSH, browser, and clipboard handling from state updates and rendering.
- Separate rendering helpers from message handling where practical.
- Group API commands/messages and request lifecycle logic coherently.
- Introduce narrow interfaces only where they improve testability or cancellation. Do not wrap every dependency.
- Preserve request-ID stale-response protection and Bubble Tea value-model semantics.
- Do not add backward-compatibility layers, generic repositories, global mutable state, or broad dependency injection containers.
- Do not alter the recently hardened SSH username validation or `ssh -- user@host` behavior.

Execution rules:

- Work in small, behavior-preserving steps.
- After each meaningful extraction, run targeted tests for `internal/ui`.
- Finish with `gofmt`, `go test -race ./...`, and `go vet ./...`.
- Add or update tests when moving logic exposes an untested behavior, but avoid snapshot churn and tests that only assert file organization.
- If an existing test fails, diagnose and fix the regression rather than weakening the test.
- Never commit, amend, push, reset, checkout, clean, or modify unrelated files.

Suggested order, subject to what the code reveals:

1. Extract effect helpers and process commands.
2. Extract rendering and overlay helpers.
3. Organize API command/message lifecycle code.
4. Consider screen-focused update handlers only if this materially reduces `App.Update` complexity without obscuring transitions.
5. Consider application context/cancellation as a separate, explicitly tested change rather than mixing it into a mechanical file split.

At completion, report:

- The responsibility boundaries created.
- Files changed and why.
- Any intentionally deferred decomposition.
- Exact verification commands and results.
