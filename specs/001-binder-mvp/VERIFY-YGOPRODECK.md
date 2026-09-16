# Verifying the YGOPRODeck contract

**Status: UNVERIFIED.** Every statement the importers rest on comes from
`spec.md` and prior knowledge. Nothing in this repository has ever received a
byte from YGOPRODeck: `db.ygoprodeck.com` and `images.ygoprodeck.com` are
answered with `403` at the **CONNECT** stage by the egress proxy in the
containers W2 and W3 were built in, so not one request has reached the service.

```
$ curl -sS https://db.ygoprodeck.com/api/v7/cardinfo.php?name=Dark%20Magician
curl: (56) CONNECT tunnel failed, response 403
```

This file is the hand-off: run it from a machine with egress, then either
confirm each assumption or change the one thing named in its **If it differs**
row. Do not work around a 403 by disabling TLS verification or by routing
around the proxy.

The code is shaped so a wrong assumption is cheap to correct. Every wire type
is in `pkg/ygoprodeck/types.go` and nowhere else, and `FetchAllCards` holds the
only JSON decode in the package.

---

## Before you start

```bash
export UA='binder-cardimport (+https://github.com/simeonkorchev/binder)'
export API=https://db.ygoprodeck.com/api/v7/cardinfo.php
```

The User-Agent matters: it is what `pkg/ygoprodeck` sends, it names the project
rather than any person (no PII goes upstream, `004-security.md`), and a service
that rate-limits per client will treat these checks as the importer.

Run the checks in order — A1 downloads the dump that A2–A4 then read, so
nothing below hits the API more than it must.

---

## A1 — the full dump is one unparameterised GET

**Assumed**: `GET /api/v7/cardinfo.php` with no query parameters returns every
card in one response.

```bash
curl -sS -A "$UA" -H 'Accept: application/json' \
  -D /tmp/cardinfo.headers -o /tmp/cardinfo.json "$API"

head -c 400 /tmp/cardinfo.headers
ls -l /tmp/cardinfo.json          # tens of MB is the expected order
jq '.data | length' /tmp/cardinfo.json
```

**Confirmed if**: the status is `200`, the body parses, and `.data | length` is
in the region of 13,000 — the whole database, not a page of it.

**If it differs**
| Reality | Change |
|---|---|
| A different path or required parameters | `DefaultCardInfoURL` in `pkg/ygoprodeck/client.go` |
| The response is **paged** (a `meta`/`next` field, or a suspiciously round `length`) | This is the expensive one: `FetchAllCards` is a single request by design. Add the page loop *inside* `FetchAllCards` so `cmd/cardimport` still sees one call returning every card, and keep pacing each page through `c.limiter`. Update assumption A1 in `pkg/ygoprodeck/doc.go` and the "one request" claim in `plan.md`. |
| It requires an API key | Add it to `ygoprodeck.Config` and thread an env var through `cmd/cardimport/main.go`'s `config`. Never hard-code it (`004-security.md`). |

---

## A2, A3, A4 — the response shape

**Assumed**: a JSON object with one `data` array; each card carries `id`
(integer passcode) and `name`; a card carries `card_sets` of
`{set_name, set_code, set_rarity}` and `card_images` of
`{id, image_url, image_url_small}`.

```bash
# A2: the envelope is exactly one "data" array.
jq 'keys' /tmp/cardinfo.json

# A3/A4: the field names and types on one known card.
jq '.data[] | select(.name == "Dark Magician")
     | {id, name, sets: .card_sets[0], image: .card_images[0]}' /tmp/cardinfo.json

# id must be a number, not a string: it is the upsert key and a bigint column.
jq '[.data[].id | type] | unique' /tmp/cardinfo.json

# Every key any card carries, so a renamed field cannot hide behind a sample.
jq '[.data[] | keys[]] | unique' /tmp/cardinfo.json
jq '[.data[].card_sets[]? | keys[]] | unique' /tmp/cardinfo.json
```

**Confirmed if**: `keys` is `["data"]`, `id | type` is `["number"]`, and the
set objects carry exactly `set_name`, `set_code`, `set_rarity`.

**If it differs**: change the `json:` tags in `pkg/ygoprodeck/types.go` — they
are the only place the upstream spelling appears. Then update
`pkg/ygoprodeck/testdata/cardinfo_sample.json` to match, because that fixture is
what both the decode specs and the `cmd/cardimport` specs run on, and update
the table in its `README.md`. A field that disappears entirely is a bigger
decision: `set_rarity` is part of a printing's identity in the schema
(`card_printings_card_code_rarity_key`), so losing it is a migration, not a tag
change.

### While you have the dump: two things the importer already handles

Worth confirming, because they are the cases the skip counters exist for.

```bash
# Codes that violate card_printings_set_code_shape (^[^-]+-.+$).
# The importer skips, counts and logs these; it must never be "fixed" by
# loosening the CHECK constraint.
jq -r '[.data[].card_sets[]?.set_code] | map(select(test("^[^-]+-.+$") | not)) | unique' /tmp/cardinfo.json

# Printings with a blank rarity — also skipped, for the same reason.
jq '[.data[].card_sets[]? | select(.set_rarity == "")] | length' /tmp/cardinfo.json

# One code printed at two rarities. If this is empty, the (card_id, set_code,
# rarity) upsert key is over-engineered; if it is not, (card_id, set_code)
# would have collapsed real printings.
jq -r '[.data[] | .card_sets[]? | .set_code] | group_by(.) | map(select(length > 1)) | .[0][0]' /tmp/cardinfo.json
```

Compare the counts with what a real run reports: `cardimport` logs
`printings skipped` at WARN, one line per reason with example codes. **A skip
count far larger than what these commands predict means the dump changed shape
and the importer is quietly dropping cards** — that is exactly what the
counters are there to make visible.

---

## A5 — the rate limit is about 20 requests per second

**Assumed**: the service asks for no more than ~20 req/s.

This is the assumption with the weakest provenance and the one with a real
cost if it is wrong (a ban). Read the published guidance first, and only then
measure:

```bash
# 1. The documentation is the source of truth; the number below is only a check.
#    https://ygoprodeck.com/api-guide/  (read it; do not infer the limit)

# 2. A polite measurement: 20 sequential small requests, timed.
time (for i in $(seq 1 20); do
        curl -sS -o /dev/null -w '%{http_code} ' -A "$UA" \
          "$API?name=Dark%20Magician"
      done)
```

**Confirmed if**: all 20 return `200` and no `429` appears. A `429`, a
`Retry-After` header, or a rate-limit header (`X-RateLimit-*`) is the real
answer and beats the assumption.

**If it differs**
| Reality | Change |
|---|---|
| A different documented rate | `DefaultRequestsPerSecond` in `pkg/ygoprodeck/client.go`, and `CARD_IMAGES_INTERVAL`'s default in `cmd/cardimages/main.go` (they are separate hosts and separate limits — do not assume one number covers both). |
| `429` before 20 requests | Lower both, and consider honouring `Retry-After`: neither limiter does today. `pipeline.Limiter` is the place, and its specs already drive a fake clock, so the behaviour is testable without waiting. |

`cmd/cardimport` makes exactly **one** request, so A5 barely constrains it. It
governs `cmd/cardimages`, which makes ~13,000.

---

## A6 — images must be self-hosted, never hotlinked

**Assumed**: images are downloaded and served from our own bucket (D3), never
linked to directly on the upstream host.

This is a **policy** question, not a technical one: a `200` from the image host
proves nothing about whether hotlinking is permitted. Read the API guide and
the terms; if they are ambiguous, ask upstream rather than inferring from what
works.

```bash
# Technical check only: the path shape cmd/cardimages assumes.
curl -sS -A "$UA" -D - -o /tmp/card.jpg \
  https://images.ygoprodeck.com/images/cards/46986414.jpg | head -20
file /tmp/card.jpg
```

**Confirmed if**: the status is `200`, `Content-Type` is `image/jpeg`, and
`file` agrees it is JPEG data.

**If it differs**
| Reality | Change |
|---|---|
| A different path or extension | `CARD_IMAGES_BASE_URL`'s default in `cmd/cardimages/main.go`; the `<id>.jpg` suffix is built in `imagefetch.Fetch`. |
| `Content-Type` is `image/png` or `image/webp` | Nothing: `pipeline.extensionFor` already handles all three and refuses anything else rather than storing an object whose extension lies. |
| Hotlinking turns out to be **allowed** | Still self-host. D3 chose it so the app does not depend on a third party's uptime or URL scheme, and `cards.image_object_key` is a key, not a URL, precisely so the bucket and CDN can change. Record the finding; do not undo the decision without a new one. |
| Self-hosting turns out to be **forbidden** too | Stop and escalate: that invalidates D3 and the `image_object_key` column, and it is a product decision, not an implementation one. |

---

## When you are done

1. Tick each assumption off in `pkg/ygoprodeck/doc.go` — it lists A1–A6 as
   assumptions and points here. Replace "unverified" with the date and what was
   checked, or correct the assumption.
2. Amend `plan.md` ("The YGOPRODeck facts this plan assumes are unverified")
   and the matching risk in `spec.md`.
3. If any shape changed, update `pkg/ygoprodeck/testdata/cardinfo_sample.json`
   **and** its `README.md` table in the same commit. Both suites run on that
   fixture; leaving it stale makes green tests that prove the wrong contract.
4. Record what you learned in `.claude/memory/` so the next session does not
   re-derive it — a binding choice (e.g. a rate limit different from the docs)
   goes in `decisions.md`.
5. Mark **T020** done in `tasks.md`. It is not done until a human has seen a
   real response body; a green test suite here only proves the code agrees with
   the fixture.
