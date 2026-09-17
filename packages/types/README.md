# @binder/types

TypeScript types for the Binder API, generated from the Go source — never hand-written.

```
openapi.json   <- make gen-spec        (go run ./cmd/binderd -openapi)
src/api.ts     <- npm -w @binder/types run generate   (openapi-typescript)
```

Both files are committed, and `make check-contract` regenerates them and fails on
any difference: a route or a field that changed in Go and not here is a red gate,
not a runtime surprise in the app.

No server is needed for either step. `cmd/binderd -openapi` renders the document
from the same route registration the server uses, with no configuration and no
database, so the contract can be checked on a CI runner that has neither.

Import the re-export, never `./api` directly:

```ts
import type { paths } from '@binder/types'
```
