# Running Binder locally

Every command below was run end to end on 2026-09-18 against a real Postgres and
a real `binderd`, and the results table is that run's actual output. The gaps at
the bottom are real; read them before judging what you see.

## What you need

- Go (the version in `go.mod`), Node 22+, PostgreSQL 16 with `pg_trgm`
- A device or simulator for the app. **Expo Go will not work** — VisionCamera
  frame processors and Apple sign-in need a dev build.

## The short version

```bash
cp .env.example .env     # optional; every value has a default
make dev                 # Postgres up, migrations applied, API listening
```

Then in another shell:

```bash
make import-cards        # the card database from YGOPRODeck
make seed-user && make token
```

`make` on its own lists every target. The rest of this page is what those do and
what to check.

## 1. Database

On a developer machine, Docker is the right answer and is what `make dev` uses:

```bash
make db-up               # docker compose up -d postgres
make migrate             # applies db/migrations to DB_URL
```

The compose file creates database **`binder`** (not `binder_test`) with
user/password `binder`. That is the Makefile's `DB_URL` default.

`tools/test-db-local.sh` exists for machines with no usable container runtime —
it boots a system Postgres and prints the export lines. **It is not the normal
path**; `make test` calls it by itself when `TEST_DATABASE_URL` is unset.

> If you run it by hand, do not write
> `eval "$(tools/test-db-local.sh)" && next-command`. `eval` returns *its own*
> exit status, so a failure inside the script still runs `next-command` — with
> an empty URL. Check `[ -n "$TEST_DATABASE_URL" ]` instead.

## 2. Card database

**This is the step that had never run anywhere** — `ygoprodeck.com` is blocked
from the environment this project was built in, so the importer was proven
against fixtures only.

```bash
make import-cards
```

It has since run for real: **14,566 cards, 44,533 printings, 10,668 sets from
one request**, which confirms the response shape and field names the importer
assumed. 12 printings were skipped as `malformed_set_code` — codes with no
hyphen at all (`DB49`, `DB14`) — counted and logged rather than silently
dropped. `specs/001-binder-mvp/VERIFY-YGOPRODECK.md` still holds the
rate-limit assumption, which nothing has confirmed.

Worth checking after an import:

```sql
-- must be empty: same card, same code, same rarity twice would be a bug
SELECT set_code, rarity, count(*) FROM card_printings GROUP BY 1,2 HAVING count(*) > 1;
-- the generated split the whole match ladder rests on
SELECT set_code, set_prefix, set_number FROM card_printings LIMIT 5;
```

Card art is a separate, resumable pass — `image_object_key IS NULL` is its whole
work set, so it can be killed and re-run freely. The app works without it.

```bash
make import-images       # rate-limited to 5 req/s; ~45 min for 13k images
```

## 3. The server

```bash
make api
```

It refuses to start without `SESSION_JWT_SECRET` (32-byte floor, enforced in
code — an empty value is rejected, not accepted) and the provider client id
lists. The Makefile supplies local defaults for all three; put real ones in
`.env` when you sign in from the app for real.

## 4. Driving the API without a device

A real sign-in needs a phone. To use `curl`, create a user row and mint a token:

```bash
make seed-user           # creates the development user, if absent
TOKEN=$(make token)      # mints a session token for it
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
