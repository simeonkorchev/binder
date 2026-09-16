# Error Creation

## Table of Contents

- [Errors as Values](#errors-as-values)
- [Error String Conventions](#error-string-conventions)
- [Creating Errors](#creating-errors)
  - [`errors.New` — static error messages](#errorsnew--static-error-messages)
  - [`fmt.Errorf` — dynamic error messages](#fmterrorf--dynamic-error-messages)
  - [Decision table: which error strategy to use](#decision-table-which-error-strategy-to-use)
- [Low-Cardinality Error Messages](#low-cardinality-error-messages)
- [Custom Error Types](#custom-error-types)
  - [Custom types that wrap other errors](#custom-types-that-wrap-other-errors)

## Errors as Values

Go treats errors as ordinary values implementing the `error` interface:

```go
type error interface {
    Error() string
}
```

This means errors are returned, not thrown. Every function that can fail returns an `error` as its last return value, and every caller must check it.

```go
// ✗ Bad — silently discarding errors
data, _ := os.ReadFile("config.yaml")

// ✗ Bad — only checking in some branches
result, err := doSomething()
fmt.Println(result) // using result without checking err

// ✓ Good — always check before using other return values
data, err := os.ReadFile("config.yaml")
if err != nil {
    return fmt.Errorf("reading config: %w", err)
}
```

## Error String Conventions

Error strings MUST be lowercase, without trailing punctuation, and should not duplicate the context that wrapping will add.

```go
// ✗ Bad — capitalized, punctuation, redundant prefix
return errors.New("Failed to connect to database.")
return fmt.Errorf("UserService: failed to fetch user: %w", err)

// ✓ Good — lowercase, no punctuation, concise
return errors.New("connection refused")
return fmt.Errorf("fetching user: %w", err)
```

When errors are wrapped through multiple layers, each layer adds its own prefix. The result reads like a chain:

```
creating order: charging card: connecting to payment gateway: connection refused
```

## Creating Errors

### `errors.New` — static error messages

```go
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
```

### `fmt.Errorf` — dynamic error messages

```go
// ✗ Avoid at log/APM boundaries — each user/tenant combo becomes a unique group
return fmt.Errorf("user %s not found in tenant %s", userID, tenantID)

// ✓ Prefer — static message; the ids travel as typed attributes on the context (pkg/eslog)
ctx = eslog.WithAttr(ctx, slog.String("user_id", userID), slog.String("tenant_id", tenantID))
return fmt.Errorf("getting user: %w", ErrUserNotFound)
```

See [Low-Cardinality Error Messages](#low-cardinality-error-messages) for why this matters.

### Decision table: which error strategy to use

| Situation | Strategy | Example |
| --- | --- | --- |
| Caller needs to match a specific condition | Sentinel error (`errors.New` as package var) | `var ErrNotFound = errors.New("not found")` |
| Caller needs to extract structured data | Custom error type | `type ValidationError struct { Field, Msg string }` |
| Error is purely informational, not matched on | `fmt.Errorf` or `errors.New` | `fmt.Errorf("connecting to %s: %w", addr, err)` |
| Need structured attrs on the log line | `eslog.WithAttr(ctx, …)` + `eslog.Error(err)` | `.claude/rules/002-go-conventions.md` §4a |

## Low-Cardinality Error Messages

APM and log aggregation tools (Datadog, Loki, Sentry) commonly group events by the logged message or exception fingerprint. When the stable log message contains variable data, every unique combination can create a separate group — dashboards become noisy and alerting breaks.

```go
// ✗ Bad at the log boundary — each file/line combo can create a unique group
fmt.Errorf("error in %s at line %d of the csv", csvPath, line)

// ✓ Good (stdlib) — static error, structured attributes at the log site
err := errors.New("csv parsing error")
// ... later, at the logging boundary:
slog.Error("csv parsing failed", "error", err, "csv_file_path", csvPath, "csv_file_line", line)

// ✓ Good (this repo) — attributes travel on the context and are emitted wherever the error is finally logged
ctx = eslog.WithAttr(ctx, slog.String("csv_file_path", csvPath), slog.Int("csv_file_line", line))
```

The plain stdlib approach scatters context: the error travels up the stack and the handler logging it may no longer have the variable data. Here `pkg/eslog` solves that: `eslog.WithAttr(ctx, …)` accumulates typed attributes on the context, and the one place that logs (`humakit.CustomErrorHandler`) emits them with the error (`.claude/rules/002-go-conventions.md` §4a).

**Static wrapping prefixes are fine** — `fmt.Errorf("fetching user: %w", err)` is low-cardinality because the prefix never changes. Dynamic context in returned errors is sometimes useful for CLI output or debugging, but production logging should keep the grouping message stable and attach IDs, paths, counts, and other variable data as structured attributes.

## Custom Error Types

Create custom error types when callers need to extract structured data from errors.

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// Usage
func validateAge(age int) error {
    if age < 0 {
        return &ValidationError{Field: "age", Message: "must be non-negative"}
    }
    return nil
}
```

### Custom types that wrap other errors

Implement `Unwrap()` so `errors.Is` and `errors.As` can traverse the chain:

```go
type QueryError struct {
    Query string
    Err   error
}

func (e *QueryError) Error() string {
    return fmt.Sprintf("query %q: %v", e.Query, e.Err)
}

func (e *QueryError) Unwrap() error {
    return e.Err
}
```
