# Security Rules

These are hard rules. Violations block merge.

## Secrets

- No `.env` files in git
- No hardcoded secrets: `sk_live_`, `WORKOS_API_KEY`, `WORKOS_WEBHOOK_SECRET`, API keys
- Use environment variables loaded at startup; validate and exit if missing

## SQL

- **Never interpolate a value into SQL.** No `fmt.Sprintf`, no concatenation, no string building with anything derived from a request, a row, or a config value.
- Always use parameterized queries (`$1`, `$2`, sqlx named params)
- Use `sqlx.In()` + `Rebind()` for IN clauses

**Permitted exception — placeholder generation.** Building the *placeholder list* for a bulk insert (`($1,$2),($3,$4),…`) is string formatting over integers, not over data, and is safe. It must:
1. interpolate only loop indices — never a value, column name, or table name;
2. pass every actual value through `args ...any`;
3. carry a comment saying so.

Stating this explicitly matters: without it the rule reads as absolute, gets broken anyway (see `StoreFoodPreferences`), and the next reader cannot tell a safe placeholder builder from an injection.

## Logging / PII

- Never log email addresses, passwords, phone numbers, or other PII
- Never log raw request bodies that may contain credentials
- Use structured logging (`slog` or `zerolog`) with typed fields

## Go panics

- `panic()` only in `main.go` and `_test.go` files
- Everywhere else: return an error

## React

- No `dangerouslySetInnerHTML` — ever
- Sanitize any user-controlled content before rendering

## Dependencies

- Never commit `node_modules/`, `dist/`, or `*.gen.go` to git
- Run `npm audit` before adding new packages
- Pin major versions in `package.json`
