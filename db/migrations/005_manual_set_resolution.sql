-- 005_manual_set_resolution.sql — the rung a person decided.
--
-- The five values of set_resolution are all rungs of the *match ladder*: they
-- say which automated lookup settled the set. A set the **user** settled in the
-- review sheet (US3) had no value of its own, so it was being written as
-- 'exact' — which means "the whole printed code matched one printing" and is
-- simply false about a set a human read off the card. Once written there is
-- nothing in the row to tell the two apart.
--
-- 'manual' is printing-bearing, like the three code rungs: it is what a slot
-- records when the user picked one of the candidates an ambiguous scan offered.
-- A user who keeps a card and leaves its set open stays 'by_name' — the card is
-- known and the set is not, which is exactly what that rung already says.
--
-- ALTER TYPE ... ADD VALUE is deliberately outside a transaction block: Postgres
-- refuses to *use* a new enum value in the transaction that added it, and
-- tools/migrate.sh applies this file with psql, so a statement outside
-- BEGIN/COMMIT commits on its own.
ALTER TYPE set_resolution ADD VALUE IF NOT EXISTS 'manual';

BEGIN;

-- Same equivalence as before, stated over the two rungs that do NOT determine a
-- set rather than the ones that do. Naming the complement means this constraint
-- never mentions 'manual' -- so it can be replaced in the same migration that
-- added the value -- and a later printing-bearing rung will not need it changed
-- again. card/model.RequiresPrinting is the Go side of the same fact.
ALTER TABLE binder_slots
    DROP CONSTRAINT binder_slots_printing_matches_resolution;

ALTER TABLE binder_slots
    ADD CONSTRAINT binder_slots_printing_matches_resolution CHECK (
        (set_resolution NOT IN ('by_name', 'unresolved')) = (card_printing_id IS NOT NULL)
    );

COMMIT;
