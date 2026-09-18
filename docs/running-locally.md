# Running Binder locally

Every command below was run end to end on 2026-09-18 against a real Postgres and
a real `binderd`, and the results table is that run's actual output. The gaps at
the bottom are real; read them before judging what you see.

## What you need

- Go (the version in `go.mod`), Node 22+, PostgreSQL 16 with `pg_trgm`
- A device or simulator for the app. **Expo Go will not work** — VisionCamera
  frame processors and Apple sign-in need a dev build.

## 1. Database

```bash
make bootstrap                       # deps + pinned linters, once
eval "$(tools/test-db-local.sh)"     # starts Postgres, exports TEST_DATABASE_URL
tools/migrate.sh "$TEST_DATABASE_URL"
```

`tools/test-db-local.sh` prefers a Debian cluster and falls back to `initdb`.
`docker compose up postgres` also works if your Docker can pull images.

## 2. Card database — the step nobody has run

`ygoprodeck.com` is blocked from the environment this was built in, so the
importer is proven against committed fixtures and **never against the live API**.
On your machine it should work:

```bash
DATABASE_URL="$TEST_DATABASE_URL" go run ./cmd/cardimport
```

One request fetches the whole dump; the upsert is idempotent, so re-running is
safe. **First walk `specs/001-binder-mvp/VERIFY-YGOPRODECK.md`** — it lists the
six unconfirmed assumptions about the API's shape and rate limit, the exact
`curl` for each, and which constant to change per outcome. A malformed printed
code is skipped and *counted*, never silently dropped, so watch the summary.

Images are a separate resumable pass (`image_object_key IS NULL` is the work
set). Locally they go to disk instead of GCS — same code path, different URL:

```bash
DATABASE_URL="$TEST_DATABASE_URL" \
CARD_IMAGES_BUCKET_URL="file:///tmp/binder-cards?create_dir=true" \
  go run ./cmd/cardimages
```

## 3. The server

```bash
DATABASE_URL="$TEST_DATABASE_URL" \
SESSION_JWT_SECRET="local-dev-secret-at-least-32-bytes-long" \
GOOGLE_CLIENT_IDS="<google client ids, comma separated>" \
APPLE_CLIENT_IDS="<apple service/bundle ids>" \
PORT=8080 go run ./cmd/binderd
```

It refuses to start if any is missing, and the secret has a 32-byte floor — an
empty `SESSION_JWT_SECRET=` is rejected rather than accepted. `GET /health`
answers `{"status":"ok"}`.

## 4. Driving the API without a device

A real sign-in needs a phone. To use `curl`, create a user row and mint a token:

```bash
psql "$TEST_DATABASE_URL" -c "INSERT INTO users(id,auth_provider,auth_subject,contact_email)
  VALUES ('99999999-9999-9999-9999-999999999999','google','local-dev','you@example.com')"

TOKEN=$(SESSION_JWT_SECRET="local-dev-secret-at-least-32-bytes-long" \
  go run ./cmd/devtoken 99999999-9999-9999-9999-999999999999)
```

`cmd/devtoken` signs through the same loader the server uses, so it grants
nothing you could not already mint holding that secret. **Local only — never
point it at a deployment's secret.**

```bash
A="Authorization: Bearer $TOKEN"; H="Content-Type: application/json"; B=http://localhost:8080

# the match ladder
curl -s -X POST $B/scans/resolve -H "$A" -H "$H" -d '{"code":"LOB-EN001"}'
curl -s -X POST $B/scans/resolve -H "$A" -H "$H" -d '{"code":"LOX-EN005"}'
curl -s -X POST $B/scans/resolve -H "$A" -H "$H" -d '{"code":"ZZZ-EN001"}'
curl -s -X POST $B/scans/resolve -H "$A" -H "$H" -d '{"code":"QQQ-EN999"}'

# a binder, and a reviewed sweep filed in one transaction
BID=$(curl -s -X POST $B/binders -H "$A" -H "$H" -d '{"name":"My Duplicates"}' | jq -r .id)
curl -s -X POST $B/binders/$BID/slots/batch -H "$A" -H "$H" -d '{"cards":[
  {"cardId":"<uuid>","cardPrintingId":"<uuid>","setResolution":"exact"},
  {"cardId":"<uuid>","cardPrintingId":null,"setResolution":"by_name"}]}'
curl -s "$B/binders/$BID?page=0" -H "$A"

# marketplace
curl -s -X POST $B/listings -H "$A" -H "$H" -d '{"binderSlotId":"<slot uuid>"}'
curl -s $B/listings                       # public: no token
curl -s $B/sellers/<user uuid>/contact -H "$A"
```

**Read `outcome`, never `resolution`, to decide what to show.** The ladder
answers `resolution: "unresolved"` both when several printings matched and when
none did — opposite situations for the user. `outcome` separates them
(`resolved` / `card_only` / `ambiguous` / `no_match`).

### What that run actually produced

| Request | Result |
|---|---|
| `LOB-EN001` | `outcome: resolved`, `resolution: exact` |
| `LOX-EN005` — prefix tail misread | recovered as `by_number` → Dark Magician at `LOB-EN005` |
| `ZZZ-EN001` — number shared by two sets | `outcome: ambiguous`, both printings offered |
| `QQQ-EN999` | `outcome: no_match`, `card: null` |
| batch of 2 where the second card does not exist | **422, and the first card did not land** — the binder still held 3 slots |
| listing the same slot twice | `201` then **`409`** |
| `GET /listings` with no token | works — browse is public by design |
| seller who shared nothing | **`200` with `email: null, phone: null`**, not a 404 |
| page 0 of a 3-card binder | 3 filled pockets, 6 nulls, `pageCount: 1` |

## 5. The app

```bash
cd apps/mobile
npx expo run:ios     # or run:android — a dev build, NOT Expo Go
```

Environment it needs:

- `EXPO_PUBLIC_API_URL` — where `binderd` is. **On a physical device this cannot
  be `localhost`**; use your machine's LAN address.
- `EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS` / `..._ANDROID` — Google issues one per
  platform. With neither set the sign-in screen reports the provider
  unavailable rather than failing silently.

Apple sign-in needs the entitlement (already in `app.config.ts`) and a paid
Apple developer account; it is limited on the simulator.

## Known gaps

- **Nobody has run the app.** No emulator or device existed where it was built,
  so **no screen has been seen, in either theme**. The dark palette is proven
  only by the WCAG contrast matrix in `src/theme/tokens.test.ts`. The logic is
  tested; the pixels are not. Expect visual problems.
- **Two provider behaviours are assumed**, and are the first things a real
  sign-in will settle:
  1. Apple must copy the request `nonce` into the token **unchanged** —
     `pkg/oidc.checkNonce` does plain equality. If Apple hashed it, *every*
     Apple sign-in would be refused.
  2. Google must accept a redirect built from the application id.
- **The OCR thresholds are reasoned, not measured**: 3 identical consecutive
  reads to accept, 3 absent reads to call a card gone — 750ms at 4fps. Foils,
  wear and lighting may want other numbers; both constants are in
  `apps/mobile/src/features/scan/lib/useStableRead.ts`.
- **The GCS path is unexercised.** `file://` and `gs://` go through the same
  `gocloud.dev/blob` call, so a local run rehearses it, but nothing is deployed.
- **CI has never executed.** `.github/workflows/ci.yml` parses; the first pull
  request is its real test.
- Open items are in `.ai/findings/open/`. The largest: four read hooks hand-roll
  one fetch state machine because the app has no React Query — an architectural
  decision for you, not a defect.
