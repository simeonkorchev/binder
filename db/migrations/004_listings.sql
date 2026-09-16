-- 004_listings.sql — a binder slot marked for sale.
--
-- A listing is a flag on a slot, not a copy of it: the card, its printing and
-- its match confidence are already on binder_slots, and the seller is the
-- binder's owner. Duplicating either here would be a second place to get them
-- wrong. Price is deliberately absent — price data is out of scope for the MVP.

BEGIN;

CREATE TABLE listings (
    id                    uuid        PRIMARY KEY,
    -- One listing per slot; unlisting deletes the row, and removing the card
    -- from the binder takes its listing with it.
    binder_slot_id        uuid        NOT NULL REFERENCES binder_slots (id) ON DELETE CASCADE,
    -- Reserved for the phase-2 Cardmarket publisher: the id of this listing on
    -- Cardmarket once it has been published there. Nothing in the MVP writes
    -- it, so it is NULL for every row; text because the id is an external
    -- opaque value. UNIQUE so a double-publish cannot bind two listings to one
    -- remote listing (many NULLs are allowed).
    cardmarket_listing_id text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT listings_binder_slot_id_key UNIQUE (binder_slot_id),
    CONSTRAINT listings_cardmarket_listing_id_key UNIQUE (cardmarket_listing_id)
);

COMMIT;
