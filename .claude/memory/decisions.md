# Decisions

## Workflow and tooling

### 2026-09-16-linter-set-omits-exhaustruct-and-wrapcheck
`.golangci.yml` enables 58 linters — spotter's set minus `exhaustruct`,
`wrapcheck`, `grouper` and `importas`, each omitted for a stated reason in the
file itself.

Two are worth knowing before re-proposing them. **`exhaustruct` is wanted** by
`002-go-conventions.md` §8a, but only scoped to the row→model structs through
`settings.exhaustruct.include`; unscoped it checks every struct literal in the
repo, which is the opposite of the rule. There are no model packages yet, so the
wave that adds the first one enables it together with its include list.
**`wrapcheck` stays off**: `000-principles.md` §8a names it as the cautionary
tale (38 suppressions for a check that was never on), so turning it on is a
reviewer's call with real code in front of them, not a default to inherit.
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
