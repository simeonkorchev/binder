# users_contact_*_not_blank accepts a tab-only or newline-only contact detail

- **Category**: bug
- **Severity**: medium
- **Path**: `db/migrations/002_users.sql`
- **Found**: 2026-09-17 (W7 / T012)

`db/migrations/002_users.sql` guards the opt-in contact columns with

```sql
CHECK (contact_email IS NULL OR btrim(contact_email) <> '')
```

`btrim(text)` with no second argument trims **spaces only**. Verified on the
suite's Postgres 16:

```
SELECT btrim(E'\t') = '' AS tab_is_blank, btrim('  ') = '' AS spaces_are_blank;
 tab_is_blank | spaces_are_blank
--------------+------------------
 f            | t
```

So `contact_email = E'\t'` satisfies the constraint and is stored. That is
exactly the state the constraint's own comment says must stay impossible: a
value that is not NULL, so every caller testing only for NULL reads the account
as having opted in, but that renders as nothing at all in the contact sheet.

**Not fixed on the spot** because W7's scope is `internal/user/**`, the
migrations are forward-only and owned by W1, and closing it is a new migration
(`005_*`) plus a data sweep for any row already stored — a schema change is not
a thing to slip into an auth PR.

**No user can reach it today.** Both layers above the column trim with
`strings.TrimSpace`, which does trim tabs and newlines:
`internal/user/model.Contact.Normalise` (refused by the service as
`service.ErrContactBlank`) and `internal/user/api`'s `Resolve` on the request
body (422 naming the field). `internal/user/service/contact_test.go` pins the
tab and newline cases. The gap is only that the database's backstop is weaker
than the code in front of it, so a future writer that skips the service — a
migration, a fixture, a `cmd/` tool — could create the row.

**Fix**: a migration that drops and re-adds both constraints with an explicit
character set, e.g.

```sql
CHECK (contact_email IS NULL OR btrim(contact_email, E' \t\r\n') <> '')
```

and a `UPDATE users SET contact_email = NULL WHERE btrim(contact_email, E' \t\r\n') = ''`
(and the same for `contact_phone`) ahead of it, so the constraint can be
validated. Pin it with the two entries removed from
`internal/user/store/user_test.go`'s blank-contact table when it lands.
