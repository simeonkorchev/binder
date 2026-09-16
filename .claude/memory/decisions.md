# Decisions

## Workflow and tooling

### 2026-09-16-linter-set-omits-exhaustruct-and-wrapcheck
`.golangci.yml` enables 58 linters — spotter's set minus `exhaustruct`,
`wrapcheck`, `grouper` and `importas`, each omitted for a stated reason in the
file itself.

**`exhaustruct` is now on**, scoped as §8a asks — W5 added the first `model/`
packages and enabled it with an `include` list of the eight structs a DB row or
a match result is assembled into. The config also names the three it
deliberately omits (`AddSlotInput`, `MoveSlotInput`, `Page`), because a type
whose fields are genuinely optional per construction produces explicit-zero
noise that hides the real signal. Extend the list when a new row→model struct
lands; do not make it a package-wide pattern.
**`wrapcheck` stays off**: `000-principles.md` §8a names it as the cautionary
tale (38 suppressions for a check that was never on), so turning it on is a
reviewer's call with real code in front of them, not a default to inherit.
`grouper` and `importas` stay off as no-ops without settings.
Evidence: `.golangci.yml` · since 2026-09-16 · verified 2026-09-16

### 2026-09-16-no-migration-runner-binary
Four forward-only SQL files did not justify golang-migrate or goose: both are a
binary to install and pin in every container, and neither adds anything over
`psql -v ON_ERROR_STOP=1 -f` plus a `schema_migrations` table. `tools/migrate.sh`
is that, and psql is already a dependency of every other database step.
Revisit if down-migrations or out-of-order versioning become real needs.
Evidence: `tools/migrate.sh` · since 2026-09-16 · verified 2026-09-16

### 2026-09-16-vision-camera-is-not-a-dependency-until-w9
D2 makes the scanner react-native-vision-camera + ML Kit, which is why
`apps/mobile` is a **dev build** (`expo-dev-client`, `expo-build-properties`
with the minSdk/deploymentTarget those libraries need) and not an Expo Go app.
The libraries themselves are deliberately **not** installed yet: an unused
native module is knip noise and an Expo-drift risk for no benefit, and W9 owns
the scanner. `app.config.ts` says in a comment where its plugin entry goes.
Evidence: `apps/mobile/app.config.ts` · since 2026-09-16 · verified 2026-09-16

### 2026-09-16-binder-api-takes-an-actorfunc-until-w7-lands
`internal/binder/api.RegisterEndpoints` takes an
`ActorFunc func(context.Context) (uuid.UUID, error)` rather than reading the
user off the context itself. Identity is the server's and `002` §8b says only
the handler reads it, but *which* server — what the session token is and how it
is verified — is the user domain's (W7), which did not exist when W5 was built.
A function parameter is the smallest seam that lets the two waves land
independently: `main.go` passes the user domain's implementation and a handler
spec passes one that answers a fixed id, with no interface that nothing
implements. An error from it is a 401.

When W7 lands, wire its resolver in at `main.go` — do not move the context key
into the binder domain.
Evidence: `internal/binder/api/api.go` · since 2026-09-16 · verified 2026-09-16

### 2026-09-16-huma-adapter-is-humago
Both domains' handlers register on `huma/v2/adapters/humago` over
`http.ServeMux`. `002` §8b says "Huma v2 on Echo" and names `humaecho`, but Echo
is not a dependency here and huma ships humago in the same module, so the
adapter costs nothing to add and one dependency to avoid. Path parameters are
huma's own `{binderId}` syntax either way, so a later move to Echo is a change
in the bootstrap only. Handler specs drive the real registered API through
`httptest`, so the adapter is covered rather than mocked.
Evidence: `internal/binder/api/api_suite_test.go` · since 2026-09-16 · verified 2026-09-16
