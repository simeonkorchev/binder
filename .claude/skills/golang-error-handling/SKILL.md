---
name: golang-error-handling
description: "Idiomatic Go error handling as this repo does it — sentinels and pkg/dataerror, wrapping with a gerund + %w at every layer, errors.Is/As, errors.Join, the single handling rule, panic only in must* builders, structured logging with stdlib slog + pkg/eslog. Apply when creating, wrapping, inspecting, or logging errors in Go code."
user-invocable: true
license: MIT
compatibility: Designed for Claude Code, Codex or similar harness, and for projects using Golang.
metadata:
  author: samber
  version: "1.3.1"
  openclaw:
    emoji: "⚠"
    homepage: https://github.com/samber/cc-skills-golang
    requires:
      bins:
        - go
    install: []
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Bash(golangci-lint:*) Bash(git:*) Agent
paths:
  - "**/*.go"
---

**Persona:** You are a Go reliability engineer. You treat every error as an event that must either be handled or propagated with context — silent failures and duplicate logs are equally unacceptable.

**Modes:**

- **Coding mode** — writing new error handling code. Follow the best practices sequentially; grep adjacent code for violations (swallowed errors, log-and-return pairs) yourself.
- **Review mode** — reviewing a PR's error handling changes. Focus on the diff: check for swallowed errors, missing wrapping context, log-and-return pairs, and panic misuse. Sequential.
- **Audit mode** — auditing existing error handling across a codebase. One category at a time in this session (creation, wrapping, single-handling rule, panic/recover, structured logging); a Routine never spawns sub-agents.


# Go Error Handling Best Practices

This skill guides the creation of robust, idiomatic error handling in Go applications. Follow these principles to write maintainable, debuggable, and production-ready error code.

## Best Practices Summary

1. **Returned errors MUST always be checked** — NEVER discard with `_`
2. **Errors MUST be wrapped with context** using `fmt.Errorf("{context}: %w", err)`
3. **Error strings MUST be lowercase**, without trailing punctuation
4. **Use `%w` at every layer, including the HTTP boundary.** Internals stay out of responses through `handleErr` + the domain's `serviceErrorMappings` table (`002-go-conventions.md` §4.4), never through `%v`
5. **MUST use `errors.Is` for sentinel matching and `errors.As` for typed chain inspection** — through the `dataerror.Is<Type>Error(err)` predicates, never a raw `errors.As` at call sites (`002` §4.3). `errors.AsType` arrives with Go 1.26; `go.mod` is 1.25.6.
6. **SHOULD use `errors.Join`** (Go 1.20+) to combine independent errors
7. **Errors MUST be either logged OR returned**, NEVER both (single handling rule)
8. **Use sentinel errors** for expected conditions; when data must travel, the `pkg/dataerror` shape (`002` §4.3) — prefer a sentinel + a mapping-table row over a new type
9. **NEVER use `panic` for expected error conditions** — reserve for truly unrecoverable states
10. **SHOULD use `slog`** (Go 1.21+) for structured error logging — not `fmt.Println` or `log.Printf`
11. **No third-party error library.** Structured attributes travel on the context with `eslog.WithAttr(ctx, …)` and reach the log line through `eslog.Error(err)` (`002` §4a)
12. **HTTP logging is centralised** — `humakit.CustomErrorHandler` (5xx at ERROR, <500 at DEBUG) and `otelecho` tracing; never add request-logging middleware or hand-written spans (`002` §4a)
13. **Use log levels** to indicate error severity
14. **Never expose technical errors to users** — translate internal errors to user-friendly messages, log technical details separately
15. **Keep log grouping low-cardinality** — at logging/APM boundaries, keep message templates stable and attach IDs, paths, line numbers, and counts as structured attributes. Error values may include useful operational context, but avoid putting high-cardinality data into the stable log message used for grouping.

## Detailed Reference

- **[Error Creation](./references/error-creation.md)** — How to create errors that tell the story: error messages should be lowercase, no punctuation, and describe what happened without prescribing action. Covers sentinel errors (one-time preallocation for performance), custom error types (for carrying rich context), and the decision table for which to use when.

- **[Error Wrapping and Inspection](./references/error-wrapping.md)** — Why `fmt.Errorf("{context}: %w", err)` beats `fmt.Errorf("{context}: %v", err)` (chains vs concatenation). How to inspect chains with `errors.Is`, `errors.As`, and Go 1.26+ `errors.AsType` for type-safe error handling, and `errors.Join` for combining independent errors.

- **[Error Handling Patterns and Logging](./references/error-handling.md)** — The single handling rule: errors are either logged OR returned, NEVER both (prevents duplicate logs cluttering aggregators). Panic/recover design, attributes that travel with the error via `pkg/eslog`, and `slog` structured logging integration for APM tools.

## Auditing

Audit one category at a time in this session — creation, wrapping, single-handling rule, panic/recover, structured logging — and consolidate. A Routine never spawns sub-agents; interactively, only when the user asks.

## Cross-References

- → See `.claude/rules/002-go-conventions.md` §4a for `pkg/eslog` (`WithAttr`, `Error`, `LeveledErr`) and the logging contract
- → See `.claude/rules/002-go-conventions.md` §4a (logging & tracing: `slog`, `eslog`, `otelecho`) for structured logging setup, log levels, and request logging middleware
- → See `.claude/skills/golang-safety/SKILL.md` for nil interface trap and nil error comparison pitfalls
- → See `.claude/skills/golang-naming/SKILL.md` for error naming conventions (ErrNotFound, PathError)
- → See `.github/workflows/quality-gate.yml` for automated AI-driven code review in CI using these guidelines

## References

- [lmittmann/tint](https://github.com/lmittmann/tint)
- [samber/slog-sampling](https://github.com/samber/slog-sampling)
- [samber/slog-formatter](https://github.com/samber/slog-formatter)
- [samber/slog-http](https://github.com/samber/slog-http)
- [samber/slog-sentry](https://github.com/samber/slog-sentry)
- [log/slog package](https://pkg.go.dev/log/slog)
