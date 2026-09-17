# Go backend memory

## Gate and toolchain

### go-list-dot-dot-dot-picks-up-node-modules
`go list ./...` at the repo root matches Go packages **inside `node_modules/`** —
`node_modules/flatted/golang/pkg/flatted` is one, and it is enough to make
`golangci-lint run ./...` and `ginkgo run ./...` lint and test third-party code.
Go only skips directories named `testdata` or starting with `.`/`_`, so
`node_modules` is not excluded for us.

Address the first-party tree explicitly instead: the Makefile's `GO_ROOTS` is
`./internal/... ./pkg/... ./cmd/...` and every Go lane uses it. Add a new
top-level Go directory to `GO_ROOTS` and to `tools/changed-go-pkgs.sh`, or it is
silently outside the gate.
Evidence: `Makefile` (GO_ROOTS) · since 2026-09-16 · verified 2026-09-16

### empty-go-tree-fails-the-linter-and-ginkgo
With no Go package in the tree at all, `golangci-lint` exits 5
(`no go files to analyze`) and `ginkgo run` exits 1 (`Found no test suites`) —
both treat "nothing to do" as an error, so the gate would be red on a tree with
nothing wrong with it. `make lint` / `make test` guard on `go list $(GO_ROOTS)`
being empty and say so out loud. The guard stops mattering by itself the moment
the first package lands; do not remove it by hand while `internal/`, `pkg/` and
`cmd/` are still skeletons.
Evidence: `Makefile` (GO_SKIP_IF_EMPTY) · since 2026-09-16 · verified 2026-09-16

### golangci-lint-install-script-is-blocked-use-go-install
golangci-lint's own installer (`curl … raw.githubusercontent.com/golangci/…/install.sh`)
is answered with **403 by the egress proxy** in this project's containers.
`proxy.golang.org` is reachable, so `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(cat .golangci-version)`
is the one path that works both on a laptop and in an agent container. That is
what `make install-go-tools` does, and CI calls the same target so the pinned
version has one source.

The image also ships an **older golangci-lint in `/usr/local/bin`** (2.5.0). It
shadows the pinned one unless the Go bin dir comes **first**: run
`export PATH="$(go env GOPATH)/bin:$PATH"` in the same shell as the gate.
`make check-golangci-version` fails loudly rather than linting with the wrong one.
Evidence: `Makefile` (install-go-tools, check-golangci-version) · since 2026-09-16 · verified 2026-09-16

### migrations-are-applied-by-tools-migrate-sh
`db/migrations/*.sql` are forward-only, carry their own `BEGIN`/`COMMIT` and use
no `IF NOT EXISTS` on tables, so re-running one is an error on purpose.
`tools/migrate.sh <url>` applies them in filename order and records each in a
`schema_migrations` table, so "already applied" is recorded rather than guessed.
`make migrate` (dev, `DB_URL`) and `make migrate-test` (boots a database if
`TEST_DATABASE_URL` is unset) are the entry points; `migrate-test` is part of
`gate-go` because CI runs it before the suites.
Evidence: `tools/migrate.sh` · since 2026-09-16 · verified 2026-09-16

## Suites and fakes

### store-suites-take-testdb-in-the-bootstrap-not-beforesuite
`testdb.New(t)` takes a `*testing.T` — it registers cleanup on it and calls
`t.Skipf` when no Postgres is reachable — so a Ginkgo store suite cannot call it
from `BeforeSuite`, where no `*testing.T` exists. Take the handle in the suite
bootstrap, **before** `RunSpecs`, into a package-level var:

```go
//nolint:gochecknoglobals // shared test DB handle for the suite (002 section 9).
var testDB *sqlx.DB

func TestStore(t *testing.T) {
	testDB = testdb.New(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "…")
}
```

The skip then skips the whole suite (`t.Skipf` calls `runtime.Goexit`, so
`RunSpecs` never runs), which is what you want on a machine with no database.
Each spec gets its own isolation from a `TRUNCATE <tables> CASCADE` in
`BeforeEach` — the database is already private to the suite, cloned from the
migrated template, so truncation is cheaper than a tx-per-spec and survives the
store opening its own transactions.
Evidence: `cmd/cardimport/store/store_suite_test.go` · since 2026-09-16 · verified 2026-09-16

### a-describetable-inherits-an-ancestors-justbeforeeach
A `DescribeTable` nested inside a container whose `JustBeforeEach` **invokes the
method under test** runs that invocation *before* the table's body, with whatever
the enclosing fixture variables happen to hold — the zero value, since the table
entry never gets a chance to set them. The symptom is a table of "must be
refused" entries where the call count is 1 instead of 0 and nothing in the entry
explains it: the assertion is failing on an *earlier*, legitimate call.

Ginkgo is behaving as documented; the pattern is the bug. Put the table in its own
sibling container and let it call the method itself, and say in a comment why it
is not under the Describe it belongs to.
Evidence: `internal/user/service/contact_test.go` · since 2026-09-17 · verified 2026-09-17

### tamper-a-token-by-flipping-to-a-different-byte-not-to-a-fixed-one
`raw[:len(raw)-1] + "A"` does not tamper with anything on the runs where the
signature already ends in `A` — the token goes through unchanged, verifies, and
the spec asserting it is *rejected* fails. base64url makes that about 1 in 64, so
it passes locally and fails the gate later; it did, on the first `make check`.
Flip to a *different* byte instead (`'A' -> 'B'`, else `-> 'A'`). Both
tampered-signature specs here use that shape and ten consecutive runs are green.
Evidence: `internal/user/session/session_test.go` (tamperSignature) · since 2026-09-17 · verified 2026-09-17

### gochecknoglobals-fires-in-test-files-too
`.golangci.yml` excludes only `forcetypeassert`, `goconst` and `ireturn` from
`_test.go`, so **`gochecknoglobals` applies to specs**. The package-level
"static (inline init)" fixture block that `002` section 9 shows is therefore a
lint error here unless it is an `error` value (the linter exempts those, which
is why a package-level `errDB = errors.New(…)` passes and a
`fixtureBatch = model.Batch{…}` does not).

Declare spec fixtures **inside the `Describe` closure** instead — Ginkgo builds
the tree once, so `blueEyes := model.CardRow{…}` above the `var (…)` block of
dynamic vars behaves identically and needs no suppression. Reserve the
`//nolint:gochecknoglobals` for the suite-shared DB handle, where there is no
closure to hold it.
Evidence: `cmd/cardimport/store/store_test.go` · since 2026-09-16 · verified 2026-09-16

### ginkgo-run-does-fail-on-plain-go-tests
`make test` is `ginkgo run`, which reports specs — but it compiles the package's
whole test binary, so a plain `func TestX(t *testing.T)` in an
`<name>_internal_test.go` **does** run and **does** fail the gate
(`--- FAIL: TestObjectKey…` then `Test Suite Failed`), even while the spec
summary above it says `SUCCESS! -- 37 Passed`. Verified by breaking one on
purpose. The `002` section 9 licence to use plain `testing` for pure helpers and
mapper-completeness tables is therefore real coverage here, not a test that
silently never runs. Read the tail of the output, not the spec line.
Evidence: `cmd/cardimages/pipeline/objectkey_internal_test.go` · since 2026-09-16 · verified 2026-09-16

### no-pkg-eslog-here-demote-with-a-level-returning-method
`002` section 4a tells you to demote an expected error with
`eslog.LeveledErr(slog.LevelWarn, err)` and attach errors with `eslog.Error(err)`.
**`pkg/eslog` does not exist in this repository** — the rules are spotter's and
name a package that was not ported. Do not invent one for a single call site.

The shape in use instead: the closed set of failure reasons is a named type with
a `Level() slog.Level` method, and the one place that logs a per-item failure
calls `slog.LogAttrs(ctx, reason.Level(), …)`. One decision per reason, in one
place, testable as a table. A 404 from the image host is the only WARN; every
other reason is ERROR. If a second layer ever needs the same thing, that is the
moment to port `pkg/eslog`, not before.
Evidence: `cmd/cardimages/pipeline/failure.go` (`FailureReason.Level`) · since 2026-09-16 · verified 2026-09-16

## API layer (Huma)

### huma-documents-three-shapes-as-the-wrong-nullability
huma decides a property's nullability from the Go type behind it, and gets three
of this API's shapes wrong — in both directions:

- **a pointer to a struct** becomes a bare `$ref` listed in `required`, so
  `*scanCard` is documented as always present and a client that trusts the
  contract dereferences null;
- **a pointer to a named array type that unmarshals from text** — `*uuid.UUID`
  is one — becomes a plain non-null `string`, because the registry decays
  "pointer to array" to "array" before the generator ever sees the pointer;
- **every slice** is nullable (`huma.DefaultArrayNullable` is true) while every
  mapper here returns a made slice, so clients are told to expect a `null` that
  can never arrive.

`humaschema.Config` replaces `huma.DefaultConfig` at every site that builds an
API — the server, the `-openapi` generator, all four handler suites — and
corrects all three under one rule: a Go pointer means null on the wire and
nothing else does, with the `nullable` tag still winning.

**Do not fix this by decorating `huma.Registry`.** huma's `mapRegistry.Schema`
generates a struct's fields by calling **itself**, not the caller's wrapper, so
a decorator only ever sees the outermost type. The correction runs on
`Config.OnAddOperation`, the first point at which an operation's schemas all
exist; it is a function of the Go types, so running it once per operation is
idempotent. And huma **panics** on `nullable:"true"` over a field whose schema
is a `$ref` to an object, so a nullable object cannot be asked for with a struct
tag at all — it is written as `anyOf: [{$ref}, {type: null}]`.

`humaschema.Inspect` reads the rule back off the emitted document, and
`cmd/binderd/nullability_internal_test.go` fails the build on any property where
the document and the Go field disagree. Its negative control registers the same
routes on `huma.DefaultConfig` and asserts all thirteen original disagreements
are named, so a green guard cannot mean an empty one.
Evidence: `pkg/humaschema/humaschema.go` (commit 68516ba) · since 2026-09-17 · verified 2026-09-17

## Store and data

### postgres-unique-is-not-deferrable-park-rows-to-reorder-positions
`UNIQUE (binder_id, position)` is checked **as each row version enters the
index**, not at the end of the statement, so there is no single UPDATE that
shifts a dense run of positions. Even

```sql
UPDATE binder_slots SET "position" = "position" + 1 WHERE "position" >= 3
```

fails with `duplicate key value violates unique constraint
"binder_slots_binder_position_key"`: the row going 3 -> 4 meets the row still at
4. SQL gives no way to order the updates, and for a move there is no order that
works anyway — every position in the window is occupied, so every intermediate
step collides. A constraint declared with `CONSTRAINT ... UNIQUE` is not
deferrable, so `SET CONSTRAINTS ... DEFERRED` is not available either.

Do it in two statements inside one transaction: **park** the affected rows above
`max(position)`, then bring them back at their final positions. The parked range
is disjoint from the occupied one by construction and every landing position was
vacated by the first statement, so no intermediate state holds two rows at one
position. A move is the same shape with a `CASE` that sends the mover to its
destination and closes the rest up behind it. `internal/binder/store/position.go`
is the worked version, and its specs go red if the parking is removed.
Evidence: `internal/binder/store/position.go` · since 2026-09-16 · verified 2026-09-16

### store-suites-must-connect-with-pgx-or-error-translation-is-never-exercised
`cmd/` connects with the **"pgx"** driver (`_ "github.com/jackc/pgx/v5/stdlib"`).
A suite that connects with lib/pq (`sqlx.Connect("postgres", …)`) therefore runs
a different driver than production, and a store that translates driver errors
into `internal/dataerror` types silently translates nothing: the translator
matches `*pgconn.PgError` and lib/pq returns `*pq.Error`, so `errors.As` misses
and the raw error passes straight through. `internal/binder/store` shipped that
way — its conflict and invalid-reference paths were unreachable until three
specs asserted them and failed.

`internal/testdb` connects with "pgx" for this reason. pgx's stdlib driver does
apply the multi-statement migration files (no bind parameters, so the simple
protocol is used); verified from a dropped `binder_test_template`. `pq` is still
imported there for `pq.QuoteIdentifier`, which is a pure string function.
Evidence: `internal/testdb/testdb.go` · since 2026-09-16 · verified 2026-09-17

## Domains

### listings-carry-no-seller-and-reach-one-by-joining-binders
`listings` has one column of its own that matters — `binder_slot_id`, UNIQUE and
`ON DELETE CASCADE`. There is no `seller_id`, no card and no price, so every
question about a listing except "does it exist" is a join:
`listings → binder_slots → binders.owner_id` for the seller,
`→ cards` (+ `LEFT JOIN card_printings`) for what is on offer.
`internal/listing/store` therefore reads three other domains' tables, which is
deliberate: the alternative is a copy of the columns (a second place to be
wrong) or a round trip per row.

The one dependency that is *not* a join is the seller's contact details, because
they are behind consent rather than behind a key: `service.SellerContacts` is a
one-method consumer-side interface declared next to the only code that reads it,
and the user domain's service satisfies it at wiring time (002 §3a). Slot
*ownership* is not modelled that way for a plain reason worth not re-deriving:
`internal/binder/service` exposes no "who owns this slot" method to consume, and
adding one to it from outside its own wave was not on the table.

W6 and W7 landed in parallel and the two halves do not line up by name: the
consumer asks for `ContactFor(ctx, userID) (listing/model.SellerContact, error)`
and `internal/user/service` offers `SellerContact(ctx, userID) (user/model.Contact, error)`.
Both carry the same two `*string` fields and the same contract, so the bootstrap
wiring is a few-line adapter type — that is where the two are reconciled, not by
either domain importing the other.
Evidence: `internal/listing/service/service.go`, `internal/user/service/contact.go` · since 2026-09-17 · verified 2026-09-17

### a-seller-who-shared-no-contact-details-is-a-200-not-a-404
`GET /sellers/{id}/contact` answers `200 {"email":null,"phone":null}` for an
account that opted into neither field — the default row in `002_users.sql`, and
the state most accounts are in. The client's response to it is to show no
contact button, so it needs to tell that apart from a failure, and encoding it as
a 404 or an error would make every caller convert it back (000 §8b).

Only an id that names **no user at all** is `ErrSellerNotFound` → 404. The
split is stated on the `SellerContacts` interface (both-nil + nil error vs.
`dataerror.MissingEntityError`) so the real implementation and a fake cannot
drift; if the user domain's method disagrees when it lands, the adapter in the
bootstrap is what reconciles it, not a change in either domain.
Evidence: `internal/listing/service/seller.go` · since 2026-09-17 · verified 2026-09-17

## Auth and identity

### go-jose-validates-only-the-claims-it-finds
`jwt.Claims.Validate`/`ValidateWithLeeway` check `iss`, `aud`, `exp`, `nbf` and
`iat` **only when the token carries them**. Every registered claim is a pointer
or a zero-able field, so a token with **no `exp` never expires**, one with no
`iss` matches any issuer, and one with no `sub` yields an empty identity — all
three pass validation and return nil. `jwt.Expected` is no help either: an empty
`Expected.Issuer` or `AnyAudience` makes go-jose *skip* that check rather than
fail it, so a misconfigured verifier accepts everything.

`pkg/oidc.checkClaims` therefore refuses an absent `iss`, `sub`, `aud` or `exp`
explicitly before calling `ValidateWithLeeway`, and `NewVerifier` refuses an empty
issuer list or audience list at construction. Four `DescribeTable` entries in
`pkg/oidc/verifier_test.go` sign a token with one claim deleted; removing the
guard turns two of them red (verified).

Two more defaults worth knowing: `Validate`'s leeway is **one minute**, which
keeps accepting an expired token for that minute — both verifiers here call
`ValidateWithLeeway` with a 30s `clockSkew` constant instead. And
`jwt.ParseSigned` takes the permitted algorithms as an argument: provider tokens
are parsed with `{RS256, ES256}` only, because accepting an HMAC algorithm is the
algorithm-confusion attack (sign your own claims with the provider's *public* key
as the shared secret), and session tokens with `{HS256}` only.
Evidence: `pkg/oidc/oidc.go` (checkClaims) · since 2026-09-17 · verified 2026-09-17

### apple-jwks-is-403-here-google-is-reachable-and-parses
`https://appleid.apple.com/auth/keys` is answered **403 by the egress proxy** in
this project's containers. `https://www.googleapis.com/oauth2/v3/certs` is
reachable: fetched with the real `oidc.NewHTTPFetcher(...).Fetch` it returns 2
keys, both `alg=RS256 use=sig` and `JSONWebKey.Valid() == true`, so that constant
and the fetcher are proven against the live provider. Apple's is not, and **no
live sign-in with either provider has ever been exercised** — both providers'
issuers and audiences are configuration nobody has confirmed against a real
token.

So JWKS fetching is behind `oidc.Fetcher` and every spec generates an RSA key
pair, signs its own token and serves the key set from `httptest`. Do not add a
spec that reaches a provider; it would pass on a laptop and 403 here.
`oidc.CachedKeys` caches with a TTL *and* refetches when asked for a key id it
does not hold (a rotation mid-TTL would otherwise be a sign-in outage), floored
by `MinRefetchInterval` so forged key ids cannot turn into a request each.
Evidence: `pkg/oidc/keys.go`, `internal/user/identity/identity.go` · since 2026-09-17 · verified 2026-09-17

### env-required-checks-presence-only-so-validate-the-value-too
`caarlos0/env`'s `env:"X,required"` fails only when the variable is **absent**.
`X=` — set to the empty string — parses happily, which is exactly how a
deployment ends up with an empty signing secret and no error at startup. So the
two halves are separate on purpose: `session.LoadConfig` fails on an unset
`SESSION_JWT_SECRET`, and `session.New` fails on an empty or under-32-byte one
(`ErrSecretMissing` / `ErrSecretTooShort`). **There is no fallback secret, not
even a dev default** — a fallback is how a known key reaches production. Both
paths have a spec, and the "not set" spec unsets the variable itself rather than
trusting the ambient environment.
Evidence: `internal/user/session/session.go` · since 2026-09-17 · verified 2026-09-17

### the-actor-seam-is-middleware-plus-actorfromcontext-in-user-api
`internal/user/api.Middleware(verifier)` is plain `func(http.Handler) http.Handler`
and only **authenticates**: it resolves a `Bearer` token to a user id and puts it
on the request context, and on *any* failure — no header, wrong scheme, forged or
expired token — it passes the request through with **no actor** rather than
rejecting it. Rejecting there would mean keeping a list of public paths inside the
middleware (sign-in and the listings feed are public by design), which is a second
place for the routing table to be wrong. `api.ActorFromContext` is the
`ActorFunc` all three domains take and is where the 401 happens; it also refuses
`uuid.Nil`, which would otherwise be the actor for every `owner_id` row.

The context key stays in `internal/user/api` — that is the binding decision
`2026-09-16-binder-api-takes-an-actorfunc-until-w7-lands` asked for. The seam is
pinned rather than described: `var _ binderapi.ActorFunc = api.ActorFromContext`
(and the listing one) plus a spec that drives the real `POST /binders` through
this middleware with a real signed session. **Nothing in the middleware logs** —
a rejected token is attacker-controlled bytes, an accepted one is a bearer
credential — and a spec asserts the log buffer is empty after a rejection.
Evidence: `internal/user/api/actor.go`, `internal/user/api/actor_test.go` · since 2026-09-17 · verified 2026-09-17

### pgx-error-strings-exclude-postgres-detail-so-a-500-log-cannot-leak-a-value
`internal/user/api.handleErr` logs `err.Error()` on the unmapped-500 path, same as
the binder domain. In a domain whose columns are an email address and a phone
number that is worth checking rather than assuming, so it was: a CHECK violation
on `contact_email` produces

```
setting user contact details: ERROR: new row for relation "users" violates check constraint "users_contact_email_not_blank" (SQLSTATE 23514)
```

`pgconn.PgError.Error()` is severity + **Message** + SQLSTATE and omits `Detail`,
which is where Postgres puts the offending row's values — so a constraint
violation cannot put a contact detail or an `auth_subject` in a log line. The
other half of the guarantee is code, not the driver: no error message in
`internal/user/**` interpolates a contact value or a token, and the blank-contact
error names the *fields* (`model.ContactField`) instead.
Evidence: `internal/user/api/errors.go`, verified by inspecting a live 23514 · since 2026-09-17 · verified 2026-09-17

### go-trimspace-and-postgres-btrim-disagree-about-tabs
`btrim(text)` with no second argument trims **spaces only**, so
`CHECK (contact_email IS NULL OR btrim(contact_email) <> '')` in `002_users.sql`
accepts `E'\t'`. `strings.TrimSpace` does trim tabs and newlines, so the service
and the API boundary catch what the column does not, and the column's backstop is
weaker than the code in front of it. Full write-up, blast radius and the fix
migration: `.ai/findings/open/2026-09-17-contact-blank-check-trims-spaces-only.md`.
Reach for `btrim(col, E' \t\r\n')` in any new CHECK of this shape.
Evidence: verified on the suite's Postgres 16 · since 2026-09-17 · verified 2026-09-17
