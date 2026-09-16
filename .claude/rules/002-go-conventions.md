---
paths:
  - "internal/**"
  - "pkg/**"
  - "cmd/**"
  - "tests/**"
---

# Go Conventions

Everything below is enforced by `make lint` where a linter exists for it (`.golangci.yml`, 58 explicitly enabled linters plus 3 formatters, `default: none`). Four that spotter enabled are deliberately off here, each with its reason written at the bottom of `.golangci.yml` — `exhaustruct` until the first `model/` package exists to scope it to (§8a), and `wrapcheck`, `grouper`, `importas`. Where it is not, this file is the rule. A `//nolint` is a recorded decision and must name the linter and the reason (`000-principles.md` §8a).

This file **is what the `golang-*` skills** in `.claude/skills/` were edited to agree with; it wins wherever they overlap (they say so themselves); use them for technique, use this file for the target shape.

## 1. Style — "Line of Sight"

Happy path stays left-aligned. Return early on errors or edge cases.

```go
// ❌ Nested success path
if err := validate(user); err == nil {
    if token, err := generateToken(user); err == nil {
        return token, nil
    } else {
        return "", err
    }
} else {
    return "", err
}

// ✅ Left-aligned happy path
if err := validate(user); err != nil {
    return "", fmt.Errorf("validating user: %w", err)
}
token, err := generateToken(user)
if err != nil {
    return "", fmt.Errorf("generating token: %w", err)
}
return token, nil
```

Other style rules:
- `gofmt` + `go vet` on every save; `golangci-lint` before commit. Formatters are `gci` + `gofmt` + `goimports`.
- Line length 120 (`lll`). Long signatures break one parameter per line; never suppress `lll` on a signature you can wrap.
- Short names in small scopes (`r` for request, `w` for writer)
- No stutter: `user.ID` not `user.UserID`
- Package names describe what they provide, not what they contain (`auth` not `authutils`)

### 1.1 Naming

- **Packages**: short, all-lowercase, no underscores; directory name == package name (`nutrition`, `dataerror`, `sqlxtx`). Generated satellites: `<pkg>fakes` (`servicefakes`, `apifakes`).
- **Files**: `snake_case.go` (`meal_log.go`, `food_admin.go`). Tests: `<name>_test.go`; white-box tests `<name>_internal_test.go`; suite bootstrap `<pkg>_suite_test.go`.
- **Types**: named by role, unqualified by package — call sites read `service.NewService`, `store.NewStore`. Per-endpoint request/response types are unexported and named `<action><Entity>Request` / `<action><Entity>Response` (`listPeriodsRequest`).
- **Receivers**: 1–3 letters from the type (`s *Service`, `st *Store`, `h *Handler`), never `this`/`self`.
- **Recurring short names**: `ctx`, `svc`, `st`, `tx`, `actorID`, `targetUserID`, `orgID`.
- **Import aliases** when generic layer names collide, `<domain><layer>`: `foodservice`, `userstore`, `habitsservice`. The alias is lowercase, no underscore.
- **Interfaces**: name by role (`Store`, `PermissionService`, `TargetStore`); the `-er` form only when the name genuinely is an agent (`OrgResolver`, `BarcodeFoodGetter`, `Refresher`).

### 1.2 Constructors

`NewX` returns a **pointer**; dependencies are injected as interfaces; the field name mirrors the parameter:

```go
type Service struct {
    store      Store
    permission PermissionService
}

func NewService(store Store, permission PermissionService) *Service {
    return &Service{store: store, permission: permission}
}
```

- Bare `New` only when the package name already carries the meaning (`appctrl.New()`).
- `revive` caps arguments at 5 and results at 3. Past that, pass an input struct from `model/` (`model.CreatePeriodInput`) — never grow the parameter list and never suppress the linter.
- Value types (enums, small value objects) get value-returning constructors.

### 1.3 Receivers

- **Pick one receiver kind per type and never mix.** `Scan`/`UnmarshalJSON` must be pointer receivers, so a type that implements them uses pointer receivers throughout.
- New `Service`/`Store` types use **pointer receivers** (their constructor already returns a pointer). When adding a method to an existing type, use whatever that type already uses — several domains use value receivers on `Service`/`Store`; that is fine, mixing is not.
- Value receivers are otherwise reserved for small immutable value types (enums, `dto`-style dates).

## 1a. Shape — no accidental complexity

The checklist in `007-clean-code-checklist.md` holds every session to these; the sweeps enforce the same list.

- **No stringly-typed parameters.** A `string` whose legal values are enumerable is a named type with constants (`model.HealthKind`), validated once at the boundary.
- **No `map[string]any` / `any` where the keys are known** — a struct. `any` is for genuinely dynamic input only.
- **No speculative shape**: no pass-through function, one-field wrapper struct, boolean flag parameter, parameter every caller passes identically, or interface method nothing calls (`000-principles.md` §8).
- **No work thrown away**: a sort repeated, a map built only to be ranged back into a slice, a 1:1 mapper between identical shapes.
- **Modern stdlib idioms** over hand-rolled loops: `min`/`max`, `slices`, `maps`, range-over-int (Go ≥ 1.21; the `golang-modernize` skill lists them — judge by reading, no analyzer).
- **Dead code is deleted**, with the zero-reference proof, never commented out or kept "in case".
- **Shared helpers first**: an `InTx` closure that returns a value is `storetx.Value` (§7); a date parse `pkg/dateparse` already has is not rewritten.

## 2. File size

Target 400 lines, hard cap 500. Above cap almost always signals mixed responsibilities. Split by domain concern:

| What to extract | Where it goes |
|-----------------|---------------|
| SQL queries | New method in `store/` |
| Business logic branch | New function in `service/` |
| Request/response parsing | New file in `api/` |

**Exceptions** (do not split): auto-generated files, test files.

**Enforced in CI** by the `file-size` job in `.github/workflows/ci.yml`. The 400-line target is advice; the 500-line cap fails the build. Before this job existed the cap was documentation only, and `store/food.go` sat at 600 lines with a green gate.

## 3. Interfaces & DI

- **No premature abstraction**: do not create an interface unless (a) there are ≥2 concrete implementations actively used, or (b) it is strictly required to mock an external I/O dependency (DB, HTTP, external API).
- **Accept interfaces, return structs**: functions accept interface types for decoupling but return concrete types so callers know exactly what they receive. `ireturn` enforces this; returning an interface needs `//nolint:ireturn // <why>` (e.g. a factory returning the interface the consumer declares).
- **Zero magic**: no `reflect` package and no `unsafe` in production code. `init()` is banned (`gochecknoinits`); wiring lives in `cmd/spotter/main.go`.
- No global state (`gochecknoglobals`). Inject via constructors: `NewService(repo Repository)`. The escape hatch is a read-only lookup table or a suite-shared test handle, annotated `//nolint:gochecknoglobals // <why>`.
- **Define interfaces at the point of use (consumer owns the interface).** The concrete `store.Store` structurally satisfies `service.Store`, `service.TargetStore`, etc.; each consumer declares only the methods it calls. A handler file declares the `<Concern>Service` interface it consumes next to its handlers; `api.Service` composes them by embedding (see `000-principles.md` §7-I for the size rule).
- Every consumer-side interface that a test needs to fake carries `//counterfeiter:generate . <Iface>` directly above it, and the package has exactly one `//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate` (see §8c).
- Small, focused interfaces over "God" interfaces.

## 3a. Layering & wiring

```
cmd/spotter/main.go → api → service → (Store interface, implemented by store) → model
```

- Data flows one direction: `api → service → store`. Never import upward (`001-architecture.md`). `model/` imports no other layer.
- **Wiring happens only in `cmd/spotter/main.go`**, through `must*` builder functions (`mustInitEndpoints`, `mustBuildAuthMiddleware`). Manual constructor injection — no wire/fx/dig. A long DI function is acceptable *only* there, suppressed with intent (`//nolint:funlen // DI kept in one place`).
- **Cross-domain dependencies go through a consumer-side interface** declared in the consuming `service/` and satisfied by the other domain's concrete service at wiring time in `main.go`. Importing another domain's `model/` (types) or `service/` (sentinel errors) is fine; holding another domain's concrete `*service.Service` is not.
- **`pkg/` must not import `internal/`.** `pkg/` is domain-free code that would be at home in another service (`sqlxtx`, `eslog`, `appctrl`, `humakit`). If a type mentions a Spotter noun (`User`, `Meal`, `Organization`) it belongs in `internal/`. Known debt: `pkg/httpctrl`, `pkg/auth`, `pkg/ratelimit` import `internal/user` and `internal/dataerror`, and `pkg/routes` (the route table) imports every domain's `api` — do not add to that list; a new package that needs `internal/` goes under `internal/`.
- Shared kernel: `internal/common` (cross-domain value types), `internal/dataerror` (typed persistence errors, §4.3). Do not grow `common` into a dumping ground — a type used by one domain lives in that domain's `model/`.

## 3b. Bootstrap & config

- Config is `caarlos0/env`: the root `Config` struct in `main.go` carries `env:"NAME"` / `envDefault:"..."` tags; a package with its own settings owns its own `Config` struct (`gemini.Config`, `telemetry.Config`) and the root embeds it. Config is parsed once at startup and passed down — never read `os.Getenv` in business code.
- Startup failures stop the process: a `must*` builder logs and `panic`s/exits. This is the **only** place `panic` is acceptable outside `pkg/` `Must*` helpers and tests (§4.5, `004-security.md`).
- Lifecycle is `pkg/appctrl`: `appCtrl.Process(fn)` for long-running loops, `PeriodicProcess(interval, fn)` for pollers, `TriggeredProcess(name, fn)` + `Trigger(name)` for on-demand work, `OnShutdown(fn)` for cleanup (shutdown callbacks get a fresh, timeout-boxed context), `appCtrl.BlockMain()` as the last line of `main`. Feature code exposes a **blocking** `Run(ctx)` and hands it over; it never calls `go` itself (§5).

## 4. Error Handling

- **Never ignore errors** with `_` unless it is mathematically impossible to fail.
- **Use `errors.Is` / `errors.As`**, not direct equality (`err == ErrNotFound`), to handle wrapped errors correctly (`errorlint`).
- Empty result sets are not errors (`000-principles.md` §8b). Not-found errors are for fetching one specific entity by id.

### 4.1 Sentinel errors

Package-level `Err...` sentinels via `errors.New` (`err113` forbids ad-hoc `errors.New`/`fmt.Errorf` in comparisons), grouped in one `var (...)` block per package. Messages are lowercase, un-punctuated, and describe the condition, not the type (`errname` enforces the `Err`/`err` prefix):

```go
var (
    ErrPeriodNotFound = errors.New("period not found")
    ErrPeriodConflict = errors.New("period conflicts with existing period")
    errNoOrg          = errors.New("no organization found")   // package-private
)
```

- **Export a sentinel only when a caller must match it** — the HTTP mapping table or another package. Otherwise keep it unexported (`err...`).
- Every exported service sentinel must have a decided HTTP status in that domain's `api/errors.go` table, guarded by the `sentinelscan`-based totality test (`000-principles.md` §8c). Adding a sentinel without a table row fails the build.
- Known debt: a handful of `Scan`-related sentinels carry capitalised, type-qualified messages (`"QuestionType.Scan: expected string"`). Fix them when you touch the file; do not copy the style.

### 4.2 Wrapping

`fmt.Errorf` with `%w`, always. The message is a lowercase **gerund phrase naming the operation that failed**, not the layer or the function name:

```go
err := s.store.CreatePeriod(ctx, input)
if err != nil {
    return nil, fmt.Errorf("creating macro period: %w", err)
}
```

- A wrapper string repeated inside one function/file is hoisted into a local `const errWrapper = "...: %w"`.
- Joining a sentinel with its cause uses double-`%w`: `fmt.Errorf("%w: %w", ErrPeriodConflict, err)`. Both remain matchable with `errors.Is`.
- Wrap at every layer boundary (store → service → api) so the final message reads as a call chain: `getting macro periods: loading daily targets: sql: no rows`. Do not wrap twice inside one layer for the same call.

### 4.3 Typed data errors (`internal/dataerror`)

Persistence conditions the service must branch on are **typed errors**, not strings. Each follows the same five-part template (see `dataerror/missingentity.go`):

1. struct with unexported fields;
2. `Wrap<Type>Error(...)` constructor;
3. `Error()`;
4. `Unwrap()`;
5. `Is<Type>Error(err) bool` predicate built on `errors.As`.

Callers match through the predicate (`dataerror.IsMissingEntityError(err)`), never through raw `errors.As`. The store translates driver errors at the boundary:

```go
if err != nil {
    if errors.Is(err, sql.ErrNoRows) {
        return nil, dataerror.WrapMissingEntityError("meal", err)
    }
    return nil, fmt.Errorf("getting meal by id: %w", err)
}
```

`sql.ErrNoRows` never escapes the store layer: for a by-id lookup it becomes a `MissingEntityError`; for a collection query it becomes an empty slice (§4, `000-principles.md` §8b).

### 4.4 Domain error → HTTP mapping

Handlers **do not choose status codes**. Each domain's `api/errors.go` holds one declarative table `serviceErrorMappings` (sentinel → status + client message, or `serverFault: true` for a deliberate 500) and one `handleErr(err)` that walks it with `errors.Is`. The handler wraps with a gerund and delegates:

```go
plans, err := svc.GetMacroPeriods(ctx, actor.ID, req.UserID)
if err != nil {
    return nil, handleErr(fmt.Errorf("getting macro periods: %w", err))
}
```

Unmatched errors fall through as opaque 500s — internal detail (SQL, file paths, upstream bodies) never reaches the client. Framework-level translation (`pkg/httpctrl`, `pkg/humakit.CustomErrorHandler`) handles the cross-cutting cases (unauthorized, `MissingEntityError` → 404).

### 4.5 Panics

- `panic` is for **programmer-error assertions on unreachable branches**, never for I/O, user input, or control flow. In `internal/` that means: nowhere outside `main.go` startup builders and `_test.go` (`004-security.md`).
- The documented exceptions live in `pkg/`: `sqlxtx.MustGetTx` (called outside a transaction), `humaecho` registration (misconfigured operation group). A new `Must*` helper needs the same shape — a bug that cannot happen at runtime if the wiring is right — and a comment saying so.
- Recovery is installed once by `echoserver` (`middleware.Recover()`), not per handler. Never `recover()` in feature code.

## 4a. Logging & tracing

- **Stdlib `log/slog` only.** No logger is injected into services; defaults are installed once by `telemetry` in `main.go`. `zerolog`, `logrus`, `fmt.Println` are not used.
- Prefer the `Context` variants whenever a ctx is in scope: `slog.WarnContext(ctx, ...)`, `slog.ErrorContext(ctx, ...)`. That is what carries request attributes and trace ids into the record.
- **Structured, typed attributes only** — `slog.String("meal_id", id.String())`, `slog.Int(...)`, `slog.Duration(...)`. Never `fmt.Sprintf` into the message, never `slog.Any` for a value that has a typed constructor.
- Keys are **snake_case** (`user_id`, `target_date`). The repo carries some camelCase keys; use snake_case for new ones and fix a key when you touch its line.
- Attach errors with `eslog.Error(err)`, not `slog.Any("error", err)`.
- No PII in logs (`004-security.md`): ids, counts and enums, never emails, names or request bodies.
- **`pkg/eslog` context enrichment** is the house pattern for store-layer context: `eslog.WithAttr(ctx, attrs...)` accumulates attributes on the ctx; `eslog.ContextualErr(ctx, err)` captures them *inside* the error so the single log line at the HTTP boundary carries them; `eslog.LeveledErr(slog.LevelWarn, err)` demotes an expected error (a bad token, a 404) so it is not logged at ERROR.
- HTTP error logging is centralised in `humakit.CustomErrorHandler` (5xx at ERROR, <500 at DEBUG). Handlers and services do **not** log an error they also return — that produces one event logged at every layer.
- **Tracing: zero hand-written spans.** Spans come from `otelecho` middleware (and any instrumented client). Do not import `go.opentelemetry.io/otel/trace` in feature code; if a span is genuinely missing, add instrumentation at the infrastructure boundary in `pkg/`.

## 5. Concurrency & context

- `context.Context` is the first argument of every IO-bound or long-running function. Never store a Context in a struct (`containedctx`).
- **The acting user is an explicit parameter, not context state.** The auth middleware puts `*model.User` in the context once; a handler extracts it once (`usermiddleware.ExtractUserFromContext` or `httpctrl.HumaRequestResponse`) and passes `actor.ID` down. Services and stores take `actorID`/`targetUserID uuid.UUID` and never read the user from ctx. Do not smuggle other business data through ctx.
- **Timeouts live at infrastructure boundaries** (HTTP clients, LLM calls, shutdown), never sprinkled through business logic.
- A detached `context.Background()` in business code is an exception that must be justified inline with a comment and `//nolint:contextcheck // <why>` (`pkg/lastactive`: the write must outlive the request). `noctx` forbids un-contexted HTTP/DB calls.
- **Feature code does not start background loops.** Long-running or periodic work is a blocking `Run(ctx)` handed to `appctrl` (`Process`, `PeriodicProcess`, `TriggeredProcess`) in `main.go`; the loop selects on its work source vs `ctx.Done()` and returns plainly on cancel (`nutrition/popularityrefresher`).
- Bounded request-scoped fan-out is fine with `errgroup.WithContext(ctx)`: every goroutine uses the group's ctx, and the caller waits before returning (`nutrition/service/target.go`). **No goroutine leaks**: every `go func()` must have an explicit termination path (errgroup, cancel, channel drain).
- **Buffered channels**: capacity must be documented and calculated (`//nolint:mnd // <why this size>`). Close channels from the sender side, never the receiver.
- **Shared state**: `sync.Mutex` / `sync.RWMutex` / `sync.Map`, kept inside the type that owns the state. Never copy a mutex.
- `make test` runs Ginkgo with `-race`; concurrent code is only as safe as the last race run.

## 6. Pointer vs Value

- Pass small structs (< 64 bytes) or slices/maps by value.
- Pass large structs or state-managing structs (Mutex, DB handles) by pointer.
- Receiver kind: §1.3.

## 7. Transactions

`pkg/sqlxtx` propagates the transaction through `context.Context`; the store exposes `InTx` and every store method runs inside it:

```go
// store/
func (s *Store) InTx(ctx context.Context, cb func(ctx context.Context) error) error {
    return sqlxtx.EnsureTx(ctx, s.db, cb, sql.LevelReadCommitted)
}

func (s *Store) GetMealByID(ctx context.Context, id uuid.UUID) (model.Meal, error) {
    var meal model.Meal
    err := s.InTx(ctx, func(ctx context.Context) error {
        tx := sqlxtx.MustGetTx(ctx)
        // tx.GetContext / SelectContext / ExecContext ...
    })
    ...
}
```

- `EnsureTx` **joins** a transaction already on the ctx or **starts** one — so a store method works alone *and* inside a service-level transaction. `sqlxtx.MustGetTx(ctx)` panics outside a tx (§4.5); `GetTx` returns nil and is for the rare method that legitimately runs either way.
- **Services orchestrate transactions as closures and never see `*sqlx.Tx`**: `s.store.InTx(ctx, func(ctx) error { ...several store calls... })`. Anything that must commit atomically goes inside one closure. `pkg/storetx.Value` is the helper for a closure that returns a value.
- Never hold a tx across a network call (LLM, blob store, WorkOS). Do the I/O first, then open the tx to persist.
- Isolation is `ReadCommitted` unless a comment explains why not.
- In service tests, neutralise the boundary with the pass-through stub: `fakeStore.InTxStub = func(ctx context.Context, cb func(context.Context) error) error { return cb(ctx) }`.

## 8. Persistence (sqlx)

Raw SQL through `sqlx` — no ORM, no query builder, no sqlc. Queries are `const` strings next to the store method that runs them, parameterised (`$1`, `$2`); see `004-security.md` for the interpolation ban.

```go
query, args, err := sqlx.In(query, ids)
query = tx.Rebind(query)
```

Row structs carry `db:` tags and live in the store package; they are mapped to `model/` types by an explicit converter (§8a). Ids are minted server-side with `uuid.New()` in the service or store — never accepted from a client for a create.

## 8a. DB row → model mapping (no silent field drops)

When a DB column maps to a struct field, wire it through **every** layer: SQL
`SELECT` → row struct (`db:` tag) → the row→model converter → response struct.
Forgetting a field silently ships a zero value to the client (e.g. a missing
`short_video_url` hid the exercise's second video). To make this a **lint
error** instead of a silent bug, `exhaustruct` is scoped to the row→model
structs (`.golangci.yml` → `settings.exhaustruct.include`; test literals are
excluded). **It is not enabled in this repo yet**: there is no `model/` package
to scope it to, and unscoped it would check every struct literal in the tree —
the opposite of this rule. The wave that adds the first row→model struct enables
it and lists that struct. Every field of those structs must be set at construction — set it to
its zero value *explicitly, with a comment* when a query deliberately omits it,
never by leaving it out. When you add a column, extend the `include` list if the
new struct should be guarded, and update all converters **and** their tests.

**Scope the guard to row→model structs, not to everything.** `include` currently
covers `workout/model.ExerciseNomenclature`, `workout/model.ExerciseTemplate`
and `nutrition/model.Food` — types assembled from DB rows, where a forgotten
field is a silent data drop. It deliberately excludes types like
`nutrition/model.MealItem` whose fields are genuinely per-construction optional
(a food item has no `MenuGroupID`); guarding those produces explicit-zero noise
that hides the real signal. When you add a struct, ask: *would leaving this
field unset ever be correct?* If yes, do not guard it.

**Always run the gate before claiming done** — `make lint && make test` compiles
the code; a reference to a non-existent field (or an unset guarded field) fails
there. Never report a backend task complete without a green gate.

## 8b. REST conventions (Huma v2 on Echo)

- **One `register<Concern>Endpoints(adapter, svc)` per handler file**, called from the domain's `api.RegisterEndpoints`. Operations are built with `newGroupOp(tag, method, path, operationID, status)` and registered with `humaecho.Register`. Operation ids are kebab-case verbs (`list-macro-periods`, `create-macro-period`); URL paths are kebab-case; every operation carries the security scheme.
- **Handlers are thin**: extract actor → `svc.X(ctx, actor.ID, req...)` → map to response. No business rules, no permission checks, no SQL in `api/`. Permission checks belong to the service (they are what the service tests assert).
- **Per-endpoint request/response types**, unexported, in the handler's file. A response is a struct with a `Body` field. **The domain model is never serialized directly** — `Body` holds an `api`-local DTO produced by a mapper (`plansToAPIPeriods`), so a model change cannot silently change the wire format (`000-principles.md` §9).
- JSON fields are **camelCase** for new types (`dayType`, `targetCalories`); the remaining snake_case fields are legacy — do not mix styles inside one struct. Path/query params use Huma tags (`path:"userID"`, `query:"from"`) and typed values (`uuid.UUID`, `time.Time`), not strings parsed by hand.
- Request-body fields are required by default; optional ones carry `required:"false"` (and a pointer type when "absent" differs from zero). Validation Huma can express goes in tags (`minimum`, `maxLength`, `enum`); conditional validation goes in a `Resolve` hook returning `huma.ErrorDetail`s (`000-principles.md` §10).
- **Identity comes from the server, never the client**: the actor from the auth context, ids from `uuid.New()` server-side, `created_at`/`updated_at` from the DB. A target user id in the path is fine (coach acting on client) — the service must check the permission.
- Errors: wrap with a gerund and return `handleErr(...)` (§4.4). Pre-service validation errors use `huma.Error4xx...` helpers directly.
- The OpenAPI spec is generated from these structs (`make gen-spec`); a route change is a contract change — run `make check-contract` and regenerate `packages/types` (`001-architecture.md`).

## 8c. Code generation

Run through `make generate` (`go generate ./...`); tools are pinned in `tools/tools.go` and vendored.

| Tool | Purpose | Convention |
|---|---|---|
| **counterfeiter** | test fakes | one `//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate` per package, plus `//counterfeiter:generate . <Iface>` above each interface; output in `<pkg>fakes/` |

- Generated directories (`*fakes/`) are never hand-edited and are excluded from lint and formatting. If a fake looks wrong, fix the interface and regenerate.
- Regenerate fakes **in the same change** that edits the interface; a stale fake fails to compile and blocks the gate.
- `go-enum` is pinned but not in use. Do not introduce a second enum mechanism ad hoc; enums today are `string`/`uint8` typed constants with `Scan`/`Value`. A new enum needs an explicit `unset`/zero value that is treated as invalid.

## 8d. Linter-enforced strictness

| Linter | Rule |
|---|---|
| `lll` | 120 columns |
| `gochecknoglobals`, `gochecknoinits` | no globals, no `init()` (§3, §3a) |
| `err113`, `errname`, `errorlint` | sentinels via `errors.New`, `Err`/`err` prefix, `errors.Is/As` (§4) |
| `ireturn`, `interfacebloat` | return structs; umbrella interfaces need a reason (§3) |
| `containedctx`, `contextcheck`, `noctx` | ctx rules (§5) |
| `exhaustruct` (scoped) | row→model structs fully constructed (§8a) |
| `revive` argument-limit 5 / function-result-limit 3 | input structs (§1.2) |
| `testpackage` | tests in `<pkg>_test`; a white-box test is `<name>_internal_test.go`, which the linter skips by filename — no directive (§9) |
| `mnd`, `goconst` | name magic numbers and repeated strings |
| `nolintlint` | every `//nolint` names the linter and the reason; unused directives are errors |

`make lint` runs the pinned version in `.golangci-version`; a different local version is refused.

## 9. Testing (Ginkgo/Gomega)

See also `006-testing.md` → "Go backend: what to test" for the layer matrix and the rules that carry the most weight.

### Runner & packaging

- `make test` = `ginkgo run -race --trace ./...`. **Specs must not depend on ordering**; use `Ordered` explicitly when state is intentionally shared.
- One suite bootstrap per package, `<pkg>_suite_test.go`: `func Test<Pkg>(t *testing.T)` + `RegisterFailHandler(Fail)` + `RunSpecs(t, "<Name> Suite")`. Dot-import `ginkgo/v2` and `gomega` in test files only.
- Tests live in the external package (`package store_test`) — `testpackage` enforces it. A white-box test of an unexported mapper/converter goes in `<name>_internal_test.go` as `package <pkg>`, with a plain comment saying what it pins. `testpackage`'s default `skip-regexp` (`(export|internal)_test\.go$`) already exempts that filename, so a `//nolint:testpackage` there is always unused and `nolintlint` fails `make lint` on it (#543 was exactly that; none of the repo's 56 internal test files carries one).
- Pure helpers and tables (mappers, sentinel-totality checks) may use plain `testing` in an internal test file instead of Ginkgo (`006-testing.md`).
- Store suites open a shared handle in `BeforeSuite` via `tests/dbhelper.GetDB` (testcontainers Postgres, or `TEST_DATABASE_URL` for an already-running disposable DB), stored in a suite global annotated `//nolint:gochecknoglobals // shared test DB handle for the suite`. Each spec runs in a tx it rolls back in `AfterEach`.
- Never run a scoped `ginkgo run ./pkg/<x>/...` (or edit a `.go` file in a package) while a full `make test` is still running in the background: both compile the suite to `<pkg>/<pkg>.test` and delete it on exit, so whichever finishes first deletes the other's binary mid-run (`fork/exec .../<pkg>.test: no such file or directory`, naming a real package and reading like a red gate on an unrelated change). Let `make test` finish before starting another `ginkgo` invocation or edit, or scope every check with `TEST_DATABASE_URL` set + `make test PKGS=./internal/<domain>/...` against the Postgres a first `make test` already booted.

### Node hierarchy
- `Describe` — subject under test
- `When` — action/trigger
- `Context` — precondition/state
- `It` — one observable outcome, all assertions here

### Variable declaration (two separate blocks)

```go
// Static (inline init)
var (
    fixture = model.Foo{ID: uuid.New()}
)

// Dynamic (no init — set in BeforeEach/JustBeforeEach)
var (
    err    error
    result model.Foo
)
```

Never mix static and dynamic in one `var` block.

### BeforeEach vs JustBeforeEach

- `BeforeEach` → setup resources, seed state, construct fakes and the SUT
- `JustBeforeEach` → call the method under test (once, shared by all Contexts)
- Never call the method under test in `BeforeEach`
- Nested `Context`/`When` blocks override stubs only; they do not re-create the SUT.

### Fakes (counterfeiter only)

- Construct in `BeforeEach` with `fakeStore = new(servicefakes.FakeStore)`; stub with `XReturns(...)`, `XReturnsOnCall(i, ...)`, `XStub = func(...)`, `XCalls(fn)`.
- The generated `servicefakes/` and `apifakes/` directories are excluded from ripgrep by `.ignore` — they were 42-66% of the hits on a method-name search and are regenerated, never edited. Use `rg --no-ignore` when you genuinely need to look inside one.
- Assert interactions with `XCallCount()` and `XArgsForCall(i)` — assert the **arguments**, not only that the call happened.
- Negative-path specs assert the store was **not** reached: `Expect(fakeStore.CreateXCallCount()).To(BeZero())`.
- Transaction boundary pass-through: `fakeStore.InTxStub = func(ctx context.Context, cb func(context.Context) error) error { return cb(ctx) }` (§7).
- Handler specs drive the real Echo/Huma stack: `newTestAdapter(fakeSvc, ...)` builds the app, `requestWithUser(method, target, userID)` puts the actor in the context, `e.ServeHTTP(rec, req)` executes. Do not call handler funcs directly.
- No hand-written mocks, no `gomock`, no `testify/mock`.

### Test data

There is no fixture package. Build inputs inline or through small local helpers (`seedCategory`, `minimalPNG`), use `pkg/ptr` for pointer fields, and keep a package-level `errDB = errors.New("db error")` (unexported, static block) as the don't-care failure.

### Cleanup

```go
AfterEach(func() {
    if tx != nil { _ = tx.Rollback() }
})
```

### Assertions

```go
Expect(err).NotTo(HaveOccurred())                        // not BeNil()
Expect(err).To(MatchError(service.ErrNotFound))          // not string compare
Expect(slice).To(HaveLen(3))                             // not len()
Expect(slice).To(ConsistOf(...))                         // order-independent
```

- Use separate variables (`dbErr`, `preErr`) for setup/verification queries.
- `DescribeTable` is for genuine matrices (serialization, status mapping), not for hiding three unrelated cases in one table.
- `Eventually` is reserved for genuinely concurrent SUTs and uses the injected-Gomega form: `Eventually(func(g Gomega) { g.Expect(...) })`. Never `time.Sleep` in a spec.
- Assert the record, not its length (`006-testing.md`).

## 10. Checklist for new Go code

- [ ] New endpoint → handler + unexported request/response DTOs in `api/<concern>.go`, consumer-side `<Concern>Service` interface with `//counterfeiter:generate`, row in `api/errors.go` for every new sentinel, `register<Concern>Endpoints` wired in `RegisterEndpoints`, `make gen-spec` + `generate-types`.
- [ ] New dependency → interface declared on the consumer side, fake generated, wired in `cmd/spotter/main.go`.
- [ ] New query → `const` SQL next to the store method, `db:`-tagged row struct, converter + completeness test, `exhaustruct` include if it is a row→model struct.
- [ ] Errors → sentinel (lowercase, un-punctuated, exported only if matched elsewhere) or `dataerror` type; wrapped with a gerund + `%w`; `sql.ErrNoRows` translated in the store.
- [ ] Background work → blocking `Run(ctx)` handed to `appctrl`; no bare goroutines.
- [ ] Logging → `slog` with `Context` variant, typed snake_case attrs, `eslog.Error(err)`, no PII, no log-and-return.
- [ ] Tests → external package, suite bootstrap, `JustBeforeEach` invokes once, counterfeiter fakes, authorization test per endpoint, `-race` green.
- [ ] `make lint && make test` green; every `//nolint` names the linter and the reason.
- [ ] `007-clean-code-checklist.md` walked over every touched file — the sweeps walk the same list, so what passes here is not refactored later.
