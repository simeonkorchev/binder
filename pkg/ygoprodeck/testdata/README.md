# `cardinfo_sample.json`

A hand-built stand-in for `GET /api/v7/cardinfo.php`, shaped like the response
the package doc assumes (**unverified — see
`specs/001-binder-mvp/VERIFY-YGOPRODECK.md`**). It is the single fixture behind
both the decode specs here and the upsert specs in `cmd/cardimport`, so the two
cannot drift apart.

Every card in it pins one thing the importer has to get right:

| Card | What it pins |
|------|--------------|
| Blue-Eyes White Dragon | `LOB-EN001` printed at **two rarities** — upserting on `(card_id, set_code)` alone would collapse them. A third, byte-identical entry pins de-duplication *within* one dump. `SDK-EN-A01` pins a **multi-hyphen** code, whose remainder must stay in `set_number`. Two `card_images` pin alternate artwork. |
| Dark Magician | `sdy-en006` pins that codes are upper-cased as imported. `PROMO` pins a **malformed** code (no hyphen) — rejected by `card_printings_set_code_shape`, so the importer must skip and count it. |
| Mirror Force | `-EN139` (empty prefix) and `MRD-` (empty number) pin the other two ways the CHECK constraint can be violated. |
| Obelisk the Tormentor | An **empty** `card_sets` — the card is imported, it just has no printings. |
| Kuriboh | A second set name under the already-seen `LOB` prefix — `card_sets` is keyed by prefix, so first-seen wins inside a batch. An empty `set_rarity` pins the "would write a blank key column" skip. |
