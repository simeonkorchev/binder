-- 001_cards.sql — the card database imported from YGOPRODeck.
--
-- A scan resolves to a *printing*: a card as printed in one set. The card is
-- the artwork-and-rules identity; the printing is what the code on the
-- cardboard names, and what the match ladder returns.

BEGIN;

-- Trusted extension since PG13; the DB owner can create it. Needed for the
-- name rung of the match ladder (see cards_name_trgm_idx below).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE cards (
    id               uuid        PRIMARY KEY,
    -- Upstream identity and the importer's upsert key: card names repeat in
    -- the YGOPRODeck dump, ids do not.
    ygoprodeck_id    bigint      NOT NULL UNIQUE,
    name             text        NOT NULL,
    -- Object key inside our bucket, not a URL: the bucket and CDN host are
    -- deployment config, and hotlinking upstream images is forbidden (D3).
    -- NULL until cmd/cardimages has fetched it, which is also how that
    -- pipeline resumes.
    image_object_key text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- The name rung of the ladder is a fuzzy match over ~13k names; a trigram GIN
-- index is what makes `name % $1` / similarity() an index scan.
CREATE INDEX cards_name_trgm_idx ON cards USING gin (name gin_trgm_ops);

-- Reference data with a natural key: the set prefix printed on every card of
-- the set ("LOB"). A surrogate uuid here would be a second key for the same
-- thing and card_printings.set_prefix could then disagree with it.
CREATE TABLE card_sets (
    code       text        PRIMARY KEY,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE card_printings (
    id         uuid        PRIMARY KEY,
    card_id    uuid        NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    -- The full printed code, e.g. 'LOB-EN005'. Stored whole because the exact
    -- rung matches on it whole.
    set_code   text        NOT NULL,
    -- Both halves are GENERATED ... STORED, not trigger-maintained columns:
    -- the split is a function of set_code, so it cannot drift from its source
    -- and no writer can forget to update it. STORED (not virtual) because
    -- these are indexed and read on every fallback lookup.
    set_prefix text        GENERATED ALWAYS AS (split_part(set_code, '-', 1)) STORED,
    -- Everything after the first '-', so a code with more than one hyphen
    -- keeps its remainder in the number rather than silently losing it.
    set_number text        GENERATED ALWAYS AS (substr(set_code, strpos(set_code, '-') + 1)) STORED,
    rarity     text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- The invariant the generated columns rely on: a printed code is a
    -- non-empty prefix, a hyphen, and a non-empty number. Rejecting a bad
    -- code at import is better than storing a printing whose halves are
    -- nonsense and which the fallback rungs would then match on.
    CONSTRAINT card_printings_set_code_shape CHECK (set_code ~ '^[^-]+-.+$'),
    -- One card can appear once per rarity under the same code (alternate
    -- rarities of a reprint share the printed code); this is the importer's
    -- upsert key.
    CONSTRAINT card_printings_card_code_rarity_key UNIQUE (card_id, set_code, rarity),
    -- The prefix is the set. Declaring it as the foreign key (rather than
    -- carrying a separate set id beside it) means a printing can never claim
    -- a set its own code does not name. NO ACTION on update because a
    -- generated column cannot be rewritten by a cascade.
    CONSTRAINT card_printings_set_prefix_fkey FOREIGN KEY (set_prefix)
        REFERENCES card_sets (code) ON UPDATE NO ACTION ON DELETE RESTRICT,
    -- Referenced by binder_slots so a slot's card and printing cannot
    -- disagree; see 003_binders.sql.
    CONSTRAINT card_printings_id_card_id_key UNIQUE (id, card_id)
);

-- One index per rung of the match ladder below the exact one.
CREATE INDEX card_printings_set_code_idx ON card_printings (set_code);
CREATE INDEX card_printings_set_prefix_number_idx ON card_printings (set_prefix, set_number);
CREATE INDEX card_printings_set_number_idx ON card_printings (set_number);

COMMIT;
