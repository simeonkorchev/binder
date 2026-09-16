# Binder

Digital trading-card binders. Scan your cards with your phone, and get a
browsable 3×3 binder you can reorganise, extend, and list for sale.

## What it does

1. **Scan** — a continuous, Apple-document-style capture. The phone detects
   the card, and on-device OCR reads the name and the set code printed just
   below the card art, in `{CODE}-{ID}` form (e.g. `LOB-EN001`).
2. **Match** — the extracted text is resolved against a Postgres card
   database, with fallbacks for imperfect OCR: full code → ID + code prefix →
   ID alone. A card matched without a code has an unknown set, and is flagged
   to the user for confirmation rather than guessed at.
3. **Organise** — matched cards land in a *binder*: ordered pages of nine
   slots. Drag to reorder, add, remove.
4. **Sell** — mark cards for sale. Other users browse what is listed and
   contact the seller.

Card data and images come from the [YGOPRODeck
API](https://ygoprodeck.com/api-guide/), imported by a rate-limit-respecting
pipeline.

**Later phase:** one-click publishing to
[Cardmarket](https://help.cardmarket.com/en/cardmarket-api).

## Status

Bootstrapping. The repo currently holds engineering guardrails only — no
application code yet. See `CLAUDE.md` for the layout, the rules, and what is
deliberately absent.

## Stack

React Native (Expo) · Go · PostgreSQL

## Working on this repo

Read `CLAUDE.md` first. It maps every rule in `.claude/rules/` to the layer it
governs, and is the orientation doc for both humans and agents.
