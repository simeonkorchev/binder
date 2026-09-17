# Binder memory index

<!-- One line per memory, pointing at its topic file and unit anchor, grouped
by topic file. Keep under 200 lines: only the first 200 lines / 25 KB load
into a session. Grep this file for the symptom in front of you — the error
text, the command, the flag — then read the unit it points at; the words in
brackets are what a session would search for, the slug is the anchor.
.claude/rules/ win over anything here. Format, placement table and rules:
.claude/memory/README.md -->

## go.md — Go backend

- [reference] `golangci-lint run ./...` or `ginkgo ./...` lints and tests `node_modules/flatted/golang`; use GO_ROOTS → go.md#go-list-dot-dot-dot-picks-up-node-modules
- [reference] "no go files to analyze" (exit 5), "Found no test suites" on the empty skeleton; the Go lanes skip on purpose → go.md#empty-go-tree-fails-the-linter-and-ginkgo
- [reference] golangci-lint install.sh answered 403 at the proxy; `/usr/local/bin` shadows the pinned binary; `make check-golangci-version` → go.md#golangci-lint-install-script-is-blocked-use-go-install
- [project] applying `db/migrations`, `schema_migrations`, `make migrate` / `make migrate-test` → go.md#migrations-are-applied-by-tools-migrate-sh
- [reference] "duplicate key value violates unique constraint" on a reorder; positions dense; UNIQUE is not deferrable → go.md#postgres-unique-is-not-deferrable-park-rows-to-reorder-positions
- [reference] dataerror translation never fires; `*pq.Error` vs `*pgconn.PgError`; which driver a suite connects with → go.md#store-suites-must-connect-with-pgx-or-error-translation-is-never-exercised
- [reference] `testdb.New` needs `*testing.T`; where a Ginkgo store suite takes its DB, `TRUNCATE ... CASCADE` per spec → go.md#store-suites-take-testdb-in-the-bootstrap-not-beforesuite
- [reference] a `DescribeTable` entry sees a call count of 1 instead of 0; an ancestor's `JustBeforeEach` fires first → go.md#a-describetable-inherits-an-ancestors-justbeforeeach
- [reference] a tampered-token spec that passes locally and fails the gate 1 run in 64 → go.md#tamper-a-token-by-flipping-to-a-different-byte-not-to-a-fixed-one
- [reference] "is a global variable (gochecknoglobals)" in a `_test.go`; where Ginkgo spec fixtures go → go.md#gochecknoglobals-fires-in-test-files-too
- [reference] `--- FAIL: TestX` under a green `SUCCESS! -- N Passed`; plain `testing` tests do run under `ginkgo run` → go.md#ginkgo-run-does-fail-on-plain-go-tests
- [project] `eslog.LeveledErr` / `eslog.Error` do not exist here; demoting an expected error (a 404) below ERROR → go.md#no-pkg-eslog-here-demote-with-a-level-returning-method
- [project] a listing has no seller_id; `listings → binder_slots → binders.owner_id`; where the cross-domain interface goes and why slot ownership is a join → go.md#listings-carry-no-seller-and-reach-one-by-joining-binders
- [project] `GET /sellers/{id}/contact` with both fields empty; 200 vs 404 for a seller; the `SellerContacts` contract → go.md#a-seller-who-shared-no-contact-details-is-a-200-not-a-404
- [reference] a token with no `exp` never expires; `jwt.Expected` with an empty issuer *skips* the check; the 1-minute default leeway; RS256-only → go.md#go-jose-validates-only-the-claims-it-finds
- [reference] `appleid.apple.com/auth/keys` 403 at the proxy, Google's certs 200 and parsed; why no spec may reach a provider → go.md#apple-jwks-is-403-here-google-is-reachable-and-parses
- [reference] `env:"X,required"` passes on `X=`; where the secret floor is enforced; no dev fallback → go.md#env-required-checks-presence-only-so-validate-the-value-too
- [project] where a handler gets the signed-in user now; `Middleware` authenticates but does not authorise; `ActorFromContext` → go.md#the-actor-seam-is-middleware-plus-actorfromcontext-in-user-api
- [reference] does a logged 500 leak a column value; `pgconn.PgError.Error()` omits `Detail` → go.md#pgx-error-strings-exclude-postgres-detail-so-a-500-log-cannot-leak-a-value
- [reference] a tab-only contact passes `btrim(col) <> ''`; `strings.TrimSpace` vs `btrim` → go.md#go-trimspace-and-postgres-btrim-disagree-about-tabs

## mobile.md — apps/mobile

- [reference] `npx expo install` → "HTTP Proxy Network Error: Forbidden"; pin versions from `bundledNativeModules.json` → mobile.md#expo-install-cannot-resolve-versions-here
- [reference] "render function has not been called" from RNTL's `screen`, "Cannot read properties of undefined (reading 'current')" from `renderHook`; v14 render and renderHook are async and need `test-renderer` → mobile.md#rntl-14-render-is-async-and-needs-test-renderer
- [reference] knip "Unlisted dependencies: expo-updates / expo-system-ui" comes from `app.config.ts` → mobile.md#knip-infers-expo-dependencies-from-app-config
- [reference] a rendered tree that is only `<RNCSafeAreaProvider />`; `getByRole('header')` finds nothing under a navigator; how to assert which tab is open → mobile.md#rendering-anything-under-the-navigator-in-jest
- [reference] "React Hook \"use\" cannot be called at the top level" from `i18next`; `import/no-named-as-default-member` on `i18n.use` → mobile.md#i18next-use-collides-with-react-19-use
- [reference] knip "Unused exported types" for a type only its own file imports → mobile.md#knip-reports-an-exported-type-no-other-file-imports
- [project] where the navigator, the locales and the theme tokens live; route names and params; why the param list is a type alias → mobile.md#where-the-app-shell-lives
- [reference] mocking `fetch` in a mobile hook test; a second call failing with a body already read; `process.env.EXPO_PUBLIC_*` set in `beforeEach` → mobile.md#jest-has-a-real-fetch-and-runtime-expo-public-env
- [project] one card scanned twenty times, or a second copy silently refused; what "left the frame" means and why it counts reads not seconds → mobile.md#a-re-read-is-a-second-copy-only-after-the-code-left-the-frame
- [project] a null `card` or `printing` the generated type says cannot be null; huma pointer fields; `@binder/types` missing from `apps/mobile/package.json` → mobile.md#the-generated-scan-match-claims-a-card-that-cannot-be-null

## decisions.md — binding

- [project] which linters `.golangci.yml` enables; `exhaustruct` include list; why `wrapcheck` is off → decisions.md#2026-09-16-linter-set-omits-exhaustruct-and-wrapcheck
- [project] where a handler gets the signed-in user before W7; `ActorFunc`; 401 → decisions.md#2026-09-16-binder-api-takes-an-actorfunc-until-w7-lands
- [project] which huma adapter; no Echo dependency; `humago` → decisions.md#2026-09-16-huma-adapter-is-humago
- [project] no golang-migrate or goose; `tools/migrate.sh` plus `schema_migrations` → decisions.md#2026-09-16-no-migration-runner-binary
- [project] why `react-native-vision-camera` is not installed yet, and what the dev build already carries → decisions.md#2026-09-16-vision-camera-is-not-a-dependency-until-w9
- [project] which listing endpoints need a session, and which deliberately do not → decisions.md#2026-09-17-browsing-listings-is-public-and-the-contact-reveal-is-not
- [project] why `GET /listings` has no `page` parameter; `listingsPerBrowse`; newest-first → decisions.md#2026-09-17-the-browse-feed-is-the-newest-50-and-has-no-paging
- [project] how long a session lasts, why 24h and not an hour, what to do when revocation is needed → decisions.md#2026-09-17-the-session-ttl-is-the-revocation-window
- [project] `PUT` not `PATCH` on the contact; how an account opts back out → decisions.md#2026-09-17-contact-details-are-replaced-not-patched
- [project] where Apple/Google issuers and client ids live; why `Issuers` is plural → decisions.md#2026-09-17-provider-facts-live-in-user-identity-not-in-pkg-oidc
- [project] why `SellerContact` returns a `dataerror` rather than its own sentinel → decisions.md#2026-09-17-sellercontact-keeps-the-stores-missingentityerror
- [project] why the scan queue calls `fetch` and not React Query; `EXPO_PUBLIC_API_URL` with no fallback; which failures are retried and which are recorded → decisions.md#2026-09-17-the-scan-queue-owns-its-fetch-there-is-no-data-layer-yet
- [project] expo-router vs react-navigation, and the five routes the MVP has → decisions.md#2026-09-17-navigation-is-react-navigation-not-expo-router

## deploy.md — infrastructure and CI

- [reference] `docker pull postgres:16` → "production.cloudfront.docker.com: Forbidden"; use `tools/test-db-local.sh` → deploy.md#docker-image-pulls-are-blocked-postgres-comes-from-the-image
- [reference] local gate vs CI drift, `go_re` / `shared_re` byte-identical, `make check-ci-parity` → deploy.md#ci-and-the-local-gate-share-one-classifier

## Derived (regenerated each session by the SessionStart hook)

_Nothing yet — `tools/memory-gen.sh` has not been ported into this repo._
