# Engineering Principles

Cross-cutting principles that apply to every layer (Go, React, React Native). Read this alongside the layer-specific rules.

---

## 1. Simplicity over Cleverness

Favor idiomatic, readable, predictable code. A mid-level engineer must be able to follow any code path without needing to ask for context.

**Forbidden patterns:**
- Macro-magic, `reflect`/`unsafe` in production Go code (see `002-go-conventions.md` §2)
- Deep nesting — use early returns ("line of sight"; see `002-go-conventions.md` §1)
- External client-side state libraries (`Redux`, `Zustand`, `Jotai`, etc.) unless explicitly requested — React Query + local state + custom hooks covers all real cases in this codebase

**Stack specifics:**
- Go: explicit `if err != nil` every time; never hide control flow (see `002-go-conventions.md` §3)
- React/RN: local state + custom hooks first; no external state library without explicit sign-off

---

## 2. Empathy for the Maintainer

Write code that a human must maintain. Self-explanatory names and structure beat documentation.

**Comments:**
- Comment **why**, never **what**. If the code needs a comment to explain what it does, rename or restructure.
- Acceptable: hidden constraint, subtle invariant, workaround for a specific upstream bug, non-obvious business rule.
- Never: "loop over items", "call the API", "set state".

**Typing:**
- No `any` in TypeScript; no untyped interfaces in Go (see `003-frontend.md` §5, `002-go-conventions.md` §2).
- Explicit contracts (interfaces/structs) for all data crossing function or API boundaries.
- Explicit return types on all custom hooks and shared API functions — inference hides breaking changes.

**File size:**
- React/RN files: target 300 lines, hard cap 400 (see `003-frontend.md` §7).
- Go files: target 400 lines, hard cap 500. A file above cap almost always mixes responsibilities — split by domain concern.

---

## 3. Fast Feedback Loops

Changes must be verifiable immediately after writing.

**Before marking any task done**, run the quality gate in `CLAUDE.md`. Lint must be clean and all tests must pass — no exceptions.

**Modularity drives testability:** keep files within the size limits above. A function or component that cannot be tested in isolation is too large or too coupled.

---

## 4. Pragmatism over Dogma

Apply patterns only when they solve a real problem in this stack (Go, React, PostgreSQL, GCS, WorkOS).

**When proposing a non-trivial solution**, state the trade-offs concisely:
- Performance impact (N+1 queries, bundle size, extra renders)
- Maintenance cost (abstraction overhead, onboarding friction)
- Fit with existing codebase patterns

**Consistency beats perfection:** if the file uses CSS variables, use CSS variables; if it uses `StyleSheet.create`, use `StyleSheet.create`. Do not introduce a new styling approach mid-file.

---

## 5. Boy Scout Rule (Continuous Debt Reduction)

Leave every file cleaner than you found it. Already stated in `CLAUDE.md` — three additions:

**No TODO hacks without permission.** If a workaround is needed, fix the root cause. A `// TODO: fix this later` left in production code requires explicit user approval.

**Fix data normalization at the source, not downstream.** String encoding issues (NFC/NFD), timezone offsets, unit mismatches — fix at the DB/API boundary. Do not paper over them with downstream patches that hide the real state.

**Fix every quality-gate failure surfaced during your work, regardless of who introduced it.** Lint errors, type errors, knip "unused" warnings, failing tests — if a gate flags them while you are running it, fix them in the same change. Do not label them "pre-existing" and move on; the next agent will hit the same noise and the project's signal-to-noise ratio erodes. If a fix is genuinely out of scope (touches an unrelated subsystem and would balloon the PR), say so explicitly and ask the user before deferring.

---

## 6. DRY — Don't Repeat Yourself

If you are about to write the same logic, utility function, or UI pattern for the second time, stop and abstract it.

- Shared business logic → custom hook (`useXxx`) or Go utility package
- Shared UI pattern → component
- Shared type → exported interface/type alias

**Exception:** code that merely *looks* identical but changes for different business reasons must stay separate. Coupling two things that evolve independently produces worse coupling than the duplication it avoids. Apply judgment: same business rule = abstract; coincidentally similar = leave separate.

---

## 7. SOLID Principles

### S — Single Responsibility
One function, component, or Go file does one thing. If a React component handles UI *and* API fetching *and* data parsing, split it: hook for logic, component for layout (see `003-frontend.md` §1 headless pattern).

### O — Open/Closed
Open for extension, closed for modification. In Go: define interfaces at the consumer; add behavior by satisfying the interface, not by modifying the implementation. In React: use compound components or render props to allow new variants without rewriting tested code.

### L — Liskov Substitution
Interface implementations must not break the behavior callers expect. In Go: if a `Store` implementation panics where the interface contract says it returns an error, it violates LSP. Test all concrete implementations against the same behavioral expectations.

### I — Interface Segregation
**An interface is owned by its consumer and contains exactly what that consumer calls.** Split by consumer, never by a method count. A handler file that uses five service methods declares a five-method interface next to itself; that is correct even though five is "a lot".

The count is a smell, not a rule: if an interface has grown past ~10 methods, ask *which consumer needs all of these?* Usually the answer is "none — it is an umbrella" and the fix is to split the **consumers**, not to renumber the methods. `interfacebloat` enforces the smell threshold; a `//nolint:interfacebloat` is acceptable only on a deliberately composed umbrella interface (e.g. `nutrition/api.Service`, which embeds per-concern role interfaces so one concrete service satisfies every handler) and must say so.

> This rule previously read "1–3 methods maximum". Nothing in the codebase obeyed it — the real role interfaces are 5–23 methods — so it produced eighteen `//nolint:interfacebloat` suppressions instead of eighteen splits. A rule that is always suppressed teaches that suppression is how you comply.

### S — Single Responsibility, for concrete types too
The SRP rule above governs functions and files. It applies to **types** as well: a struct that accumulates every method in its domain is a god object, and interface segregation cannot fix it from the outside.

- A service/store struct past ~40 methods is over its budget. Split it by concern (`FoodService`, `MealService`, …) and compose the pieces at the wiring layer.
- The generated counterfeiter fake is the fastest tell: an 8,000-line `fake_store.go` means every test in the package drags in a hundred methods it does not use.
- Splitting a live god object touches ≥2 layers, so it goes through the spec pipeline (`/spec-feature`) rather than an ad-hoc refactor.

### D — Dependency Inversion
Business logic depends on abstractions, not concretes. Go service layer accepts a `Repository` interface — it must not import a specific `postgres` package. React components depend on hook contracts, not on `useApiClient` directly (see `003-frontend.md` §1).

---

## 8. YAGNI — You Aren't Gonna Need It

Build for the present requirement. Never write code for speculative future use.

**Forbidden without explicit instruction:**
- "Just in case" utility functions, extra API fields, generic wrapper classes
- Database columns not required by the current feature
- Additional related features beyond what the prompt asks for

**Agent constraint:** if a prompt asks for feature A, implement feature A. Do not proactively build B and C because they seem related. If B and C are genuinely required, ask first.

**Refusal guardrail:** if a task description implies scope beyond what is specified, flag the ambiguity and confirm before building.

**Note the opposite failure.** YAGNI guards against building too much. The failure this codebase actually exhibits is the reverse: the same block copied until nobody notices it is a decision made in eighteen places. When you touch a block that already exists elsewhere, consolidate it — that is §6, and it is not a YAGNI violation.

---

## 8a. Suppressions Are Decisions

A `//nolint` silences a check that someone deliberately turned on. Treat each one as a recorded decision:

- **Name the linter and give the reason.** `//nolint:mnd // four placeholders per preference row`. `nolintlint` enforces both (`require-specific`, `require-explanation`), and `allow-unused: false` deletes directives that no longer suppress anything.
- **Never file-scoped.** A `//nolint` on the `package` line disables the check for every function added to that file for the rest of its life, including ones nobody has written yet. Put it on the specific line.
- **Never for a linter that is not enabled.** These accumulate silently and read as justification for code nobody checked. The repo carried 38 `//nolint:wrapcheck` for a linter that was never in the enabled list.
- **A reason that does not explain anything is worse than none** — it satisfies the tool while telling the reader nothing. `//nolint:lll` is the one exemption from `require-explanation`, because the reason is visible in the line itself.
- **Prefer fixing over suppressing.** If a signature trips `lll`, wrap it. If a function trips `funlen`/`cyclop`, it is usually doing two things.

---

## 8b. Empty Is Not An Error

A query that matches nothing returns an empty collection and a nil error. Reserve "not found" errors for fetching one specific entity by id, where absence is genuinely exceptional.

Encoding "no rows" as an error forces every caller to unwrap and convert it, and callers will disagree: in `nutrition/store` six methods shared one helper that errored on an empty set, three converted it back to an empty slice and three did not — so an empty category returned `200 []` through one method and a failure through another.

If a low-level helper must signal emptiness (e.g. to drive a fallback), keep that signal private and expose a wrapper that returns the empty slice — see `getFoods` / `getFoodsAllowEmpty`. **State the contract in the interface doc comment**, so a fake and the real store cannot drift (that is §7-L).

---

## 8c. Error Mapping Must Be Total

Every exported sentinel a service can return needs a decided HTTP status. An unmapped sentinel reaches the client as a 500 — telling them the server broke when their request was simply wrong.

- Map sentinels in **one table**, not in scattered `errors.Is` chains that each handler re-derives.
- A sentinel the client genuinely cannot act on stays a 500, but **records that choice explicitly** so "decided" is distinguishable from "forgotten".
- Back the table with a test that enumerates the sentinels **from source** and fails on any that is unlisted. A hand-copied list rots the first time someone adds an error. See `nutrition/api/errors.go` and its `TestEveryServiceSentinelIsMapped`.

---

## 9. Total Mapping at Boundaries

The same entity is re-represented at every boundary it crosses: DB row → domain model → API DTO → form/view state, and back. Each hop is usually a hand-written field-by-field copy (`toAdminFood`, `assembleFoodsFromRows`, `foodToForm`, `buildInput`). **Forgetting one field assignment compiles, passes unrelated tests, and silently drops data** — the value saves to the DB but never reads back, or never leaves the form. This is the most common "it just doesn't save / always shows blank" bug in this codebase.

Rules for any function that maps one representation of an entity to another:

- **Map every field, or omit it on purpose.** When you read or write a field on one side, account for it on the other. A field that is intentionally not carried across (e.g. `MicroNutrients` is not exposed on the admin DTO) gets a one-line comment at the mapper saying so — an omission must be a documented decision, not an oversight.
- **Each mapper needs a completeness test.** Build a source value with *every* field set to a distinct non-zero value, run the mapper, and assert *every* destination field — not just the one field your current task touches. See `006-testing.md` → "Mapper completeness tests". This is the test that turns a future dropped field into a red build instead of a production bug.
- **When you add a field to an entity, update every mapper and its completeness test in the same change.** Grep the field's siblings (e.g. search for `EnglishName` to find everywhere `EnglishBaseName` must also go) to find all the hops.
- **Prefer not hand-mapping at all** when the representations are identical — embed/alias the shared struct or reuse the type. Hand-copy only the fields that genuinely differ between the two shapes. Fewer manual assignments, fewer to forget.

---

## 10. Enforce Invariants At The Boundary

Go cannot express "exactly one of these two fields is set". When a type has fewer legal states than representable ones, something has to enforce the difference, and *"every producer sets it correctly"* is not enforcement — it holds until the one producer that doesn't is an HTTP request body.

- **Validate at the untrusted edge.** Huma calls `Resolve(huma.Context, *huma.PathBuffer) []error` on any request type that implements it; return a `huma.ErrorDetail` and the client gets a 422 naming the field. See `nutrition/api.foodItem` and `workout/api.setLogInputBody`.
- **Then fail safe inwards.** The boundary check is not a licence for the service layer to dereference blindly. A mapper that cannot resolve its input returns an error — never a zero-valued record. Writing a blank row is worse than failing: it succeeds, and the user's data is silently gone.
- **Carry the discriminant where you can.** `model.MealItem` and `api.recentItem` both carry an explicit `ItemType`/`Kind`; their *input* counterparts did not, which is where the gap was.

A concrete instance of the whole rule: `POST /meals` accepted `{"amount":150}` with neither `foodID` nor `recipe_id`, returned 200 at the boundary, and panicked one layer down on `*inputItem.FoodID`. Both halves — the `Resolve` guard and the mapper returning an error — are needed.
