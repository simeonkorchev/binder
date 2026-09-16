-- 002_users.sql — one account, its provider identity, and its opt-in contact
-- details.
--
-- D1: sign-in is Apple or Google; the session is a JWT this backend issues, so
-- the only thing we persist from the provider is the identity that lets us
-- recognise the same person again. D4: the contact button reveals an email or
-- a phone number the user chose to publish. Those two are different data with
-- different consent, so they are different columns: the provider's email is
-- deliberately not stored at all.

BEGIN;

CREATE TYPE auth_provider AS ENUM ('apple', 'google');

CREATE TABLE users (
    id            uuid          PRIMARY KEY,
    auth_provider auth_provider NOT NULL,
    -- The provider's stable subject claim ('sub'). Opaque to us, and unique
    -- only within its provider — hence the composite key below.
    auth_subject  text          NOT NULL,
    -- Both NULL is the normal state: a user who has not opted in to being
    -- contacted is fully representable and simply has no contact button.
    contact_email text,
    contact_phone text,
    created_at    timestamptz   NOT NULL DEFAULT now(),
    updated_at    timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT users_provider_subject_key UNIQUE (auth_provider, auth_subject),
    -- "Opted in" must stay distinguishable from "not": an empty or
    -- whitespace-only string would read as opted-in to every caller that only
    -- checks for NULL.
    CONSTRAINT users_contact_email_not_blank CHECK (contact_email IS NULL OR btrim(contact_email) <> ''),
    CONSTRAINT users_contact_phone_not_blank CHECK (contact_phone IS NULL OR btrim(contact_phone) <> '')
);

COMMIT;
