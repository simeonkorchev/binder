# Architecture

## Monorepo layout

```
spotter/
├── apps/trainer-web/      # Vite + React 19 + TS (coaches)
├── apps/client-mobile/    # Expo + React Native (clients)
├── packages/types/        # Auto-generated OpenAPI TS types (never hand-edit)
├── packages/api/          # Type-safe openapi-fetch client
├── packages/auth/         # WorkOS AuthKit wrappers: `@spotter/auth/web`, `@spotter/auth/native`
├── packages/intl/         # i18n setup + locales (`locales/{en,bg}.json`)
├── packages/ds/           # Design tokens, web + native
├── internal/              # Go backend (clean architecture)
├── cmd/                   # Go CLI entry points
├── pkg/                   # Shared Go packages
└── db/                    # Migrations & seed data
```

## Go domain layout (per domain)

```
internal/{domain}/
├── api/      → HTTP handlers, request/response types
├── service/  → Business logic, Store interface, servicefakes/
├── store/    → SQL queries
└── model/    → Domain structs
```

Data flows one direction: `api → service → store`. Never import upward.

## Full-stack feature checklist

1. `model/` — add/modify structs
2. `store/` — SQL query + method
3. `service/service.go` — add to Store interface
4. `service/{file}.go` — business logic
5. `api/api.go` — add to Service interface
6. `api/{file}.go` — request/response types + handler
7. `go generate ./internal/{domain}/service/...` — regenerate fakes
8. `packages/types/` — `make gen-spec && npm run generate-types` (from Go source, no server); `make check-contract` proves both are current
9. FE hook next to its feature, `apps/{app}/src/features/<feature>/` (trainer-web also keeps cross-feature hooks in `src/lib/hooks/`; client-mobile has no `lib/hooks/`)
10. FE component/screen
11. Translations in `packages/intl/locales/{bg,en}.json`

## When to use spec pipeline

Use specs/ for: features touching ≥2 layers, DB migrations, new/changed endpoints, new domain concepts.
Skip for: bug fixes, translation additions, dep bumps, renames, test-only changes.
