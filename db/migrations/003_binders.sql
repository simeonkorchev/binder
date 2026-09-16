-- 003_binders.sql — binders and their slots.
--
-- A slot's place in the binder is one flat integer. Page and slot are derived
-- (position / 9, position % 9), so a 3x3 page is a range scan and a reorder is
-- one bulk update of integers. Density of positions within a binder is the
-- reorder transaction's job — the database can only enforce uniqueness.

BEGIN;

-- How sure we are that this slot's card is the one that was scanned, one value
-- per rung of the match ladder. An enum, not a boolean: US3 has to tell the
-- user *how* the card was matched, and a boolean cannot carry that.
CREATE TYPE set_resolution AS ENUM (
    'exact',
    'by_prefix_and_number',
    'by_number',
    'by_name',
    'unresolved'
);

CREATE TABLE binders (
    id         uuid        PRIMARY KEY,
    owner_id   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Every read is "the binders of this user".
CREATE INDEX binders_owner_id_idx ON binders (owner_id);

CREATE TABLE binder_slots (
    id               uuid           PRIMARY KEY,
    binder_id        uuid           NOT NULL REFERENCES binders (id) ON DELETE CASCADE,
    position         integer        NOT NULL,
    -- A slot always names a card. The printing is what may be unknown: the
    -- name rung identifies the card but not which set it was printed in.
    card_id          uuid           NOT NULL REFERENCES cards (id) ON DELETE RESTRICT,
    -- The composite foreign key at the bottom of the table is the only
    -- reference this needs: it already proves the printing exists.
    card_printing_id uuid,
    set_resolution   set_resolution NOT NULL,
    created_at       timestamptz    NOT NULL DEFAULT now(),
    updated_at       timestamptz    NOT NULL DEFAULT now(),
    CONSTRAINT binder_slots_position_non_negative CHECK (position >= 0),
    CONSTRAINT binder_slots_binder_position_key UNIQUE (binder_id, position),
    -- The three code-based rungs resolve to a printing; the name rung and an
    -- unresolved scan do not. Stating it as an equivalence keeps the flag and
    -- the data one fact instead of two that can disagree.
    CONSTRAINT binder_slots_printing_matches_resolution CHECK (
        (set_resolution IN ('exact', 'by_prefix_and_number', 'by_number')) = (card_printing_id IS NOT NULL)
    ),
    -- When a printing is set it must be a printing *of this card*. A composite
    -- foreign key under the default MATCH SIMPLE is skipped entirely while
    -- card_printing_id is NULL, which is exactly the unresolved case.
    CONSTRAINT binder_slots_printing_belongs_to_card FOREIGN KEY (card_printing_id, card_id)
        REFERENCES card_printings (id, card_id) ON DELETE RESTRICT
);

COMMIT;
