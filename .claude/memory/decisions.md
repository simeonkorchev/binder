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

## Product and code

### 2026-09-17-browsing-listings-is-public-and-the-contact-reveal-is-not
`GET /listings` takes no `ActorFunc`: the feed is identical for everybody, it
carries no contact details, and requiring a session would be a parameter the
handler only passes on. `GET /sellers/{id}/contact` does take one — the fields
are personal data a seller published *to buyers*, and a session is the one thing
between "a buyer is asking about this card" and a script reading every seller in
the database. `POST /listings` and `DELETE /listings/{id}` need the actor to
decide ownership, so they take it for a second reason.

A maintainer may well want the feed behind a session too (D4 does not say);
flipping it is adding the parameter and one spec, and the contact reveal is the
half that must not be relaxed.
Evidence: `internal/listing/api/listing.go`, `internal/listing/api/seller.go` · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-the-browse-feed-is-the-newest-50-and-has-no-paging
`spec.md` gives the browse endpoint two parameters, `q` and `set`, and no page —
so `GET /listings` answers with the **50 most recently listed** matches
(`listingsPerBrowse`), ordered `created_at DESC, id`, and a buyer narrows with
the two filters rather than paging. The cap is in the operation summary, so it is
a stated limit and not a silent truncation, and `ListingSearch.Limit` carries it
through the service to one `LIMIT $3` — adding paging later is a query parameter
and an OFFSET, not a reshaping.

Unbounded was the alternative and was rejected: the table grows without bound
and nothing in the MVP would have kept a feed readable.
Evidence: `internal/listing/api/listing.go` (listingsPerBrowse) · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-the-session-ttl-is-the-revocation-window
A session is a JWT this backend signs (HS256) with a 24h default TTL,
`SESSION_TTL` in the environment. There is **no refresh token and no revocation
list** in the MVP, which makes that TTL the only thing that closes the window on a
stolen token — and the reason it is a day rather than an hour: with no refresh
token, a shorter session means the user re-runs Apple's or Google's sign-in sheet
during ordinary use. Symmetric rather than a key pair because the only party that
verifies the token is the party that signed it.

Shrinking it is an environment variable, not a change. If revocation becomes a
real need — a "sign out everywhere" button, a compromised account — the answer is
a `jti` plus a deny list or a `sessions_valid_from` column on `users`, and that is
when the TTL can drop.
Evidence: `internal/user/session/session.go` (DefaultTTL) · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-contact-details-are-replaced-not-patched
`PUT /me/contact` replaces the whole contact preference: a field left out, or sent
as `null`, is "not shared". PATCH was rejected because with it there is **no way to
opt back out** — absent and `null` would each have to mean both "leave this alone"
and "stop sharing this", and Go cannot tell an absent JSON field from a null one in
a `*string`. `{}` is how an account stops sharing everything, and it is a 200.

The consequence to keep in mind: a client that means to change only the phone
number must send the email back too, or it is cleared. The mobile contact form
sends both fields, which is the shape that makes this safe.
Evidence: `internal/user/api/account.go` (setContactBody) · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-provider-facts-live-in-user-identity-not-in-pkg-oidc
`pkg/oidc` is provider-agnostic on purpose — an issuer list, an audience list and a
key lookup — so it holds no mention of Apple or Google and can be tested entirely
against keys a spec generated. The two providers' issuers and JWKS URLs are
constants in `internal/user/identity`, their client ids come from
`APPLE_CLIENT_IDS` / `GOOGLE_CLIENT_IDS`, and a sign-in is dispatched through a
`map[model.AuthProvider]ProviderVerifier` rather than a switch, so a third provider
is a map entry.

`oidc.Config.Issuers` is plural because Google issues both
`https://accounts.google.com` and the bare `accounts.google.com`; accepting only
one refuses tokens at random. `Audiences` is plural because a provider issues one
client id per platform.
Evidence: `internal/user/identity/identity.go` · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-sellercontact-keeps-the-stores-missingentityerror
`internal/user/service.SellerContact` deliberately does **not** convert the store's
`dataerror.MissingEntityError` into its own `ErrUserNotFound`, unlike `GetUser`
right next to it. `internal/listing/service.SellerContacts` — the consumer-side
interface it satisfies — documents that contract and branches on
`dataerror.IsMissingEntityError`, and it is the consumer that owns the interface
(002 §3a). Converting would have moved the translation into the bootstrap adapter,
where a mistake is invisible.

The return type is the other half of the decision: `model.Contact`, not
`model.User`. The marketplace can reach a seller's published email and phone
number and has no way to reach the provider subject or the timestamps.
Evidence: `internal/user/service/contact.go` · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-navigation-is-react-navigation-not-expo-router
`apps/mobile` navigates with React Navigation (`@react-navigation/native`,
`native-stack`, `bottom-tabs`), not expo-router. Three reasons, in order of
weight: `005-mobile.md` already states the house rule in React Navigation's
vocabulary (route types in `AppNavigator.tsx` → `RootStackParamList`, a screen
typed with `NativeStackScreenProps`), so expo-router would have meant rewriting
a rule rather than following one; the entry point stays
`registerRootComponent(App)` in `index.ts`, which is what this dev-build app
(VisionCamera frame processors, `expo-dev-client`) is wired for, instead of
`expo-router/entry` plus a babel plugin and an `app/` tree; and the MVP has five
known routes, which a filesystem router's discovery buys nothing for.

Declined, not overlooked: expo-router's deep links and typed routes are the
reasons to revisit. If the marketplace ever needs a shareable link to a listing,
React Navigation's `linking` config covers it without moving the tree.

The five routes are three tabs (`Scan`, `Binders`, `Market`) and two routes
pushed over them (`BinderPage` — `{ binderId }`, `SellerContact` —
`{ sellerId }`). Neither choice requires Expo Go, which the app cannot use
anyway.
Evidence: `apps/mobile/src/navigation/AppNavigator.tsx` (commit 956ef3b) · since 2026-09-17 · verified 2026-09-17

### 2026-09-17-the-scan-queue-owns-its-fetch-there-is-no-data-layer-yet
`003-frontend.md` says React Query for all data, and `apps/mobile` has no React
Query, no API client and no query client — so `features/scan/api/useResolveScan.ts`
calls `fetch` itself rather than a dependency being added unilaterally in the
middle of a wave. The base URL is `process.env.EXPO_PUBLIC_API_URL` with **no
fallback**: unset throws, which holds the queue instead of posting scans at a
localhost nobody is serving. The second screen that calls the API is the one
that extracts a client, and that is when React Query is worth deciding on.

The queue's retry policy is part of the same decision, because it is what
React Query would otherwise own. A dropped connection or a 5xx leaves the scan
at the head of the queue (the trip failed, and it can be made again); a 4xx
leaves the queue and is recorded as rejected with its status (the resolver
answered about *that code*, and retrying it forever would wedge everything swept
after it). Draining restarts on the next accepted scan or on `retryQueued` —
there is no timer, so a queue can never spin against a server that is down.
Evidence: `apps/mobile/src/features/scan/api/useResolveScan.ts` (commit 87833f4) · since 2026-09-17 · verified 2026-09-17

