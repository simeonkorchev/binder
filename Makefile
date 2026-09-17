# Bare `make` prints the map. Set explicitly: the default goal is otherwise
# whichever target happens to appear first, which a refactor can change silently.
.DEFAULT_GOAL := help

# Development database. The test database is not here: it comes from
# TEST_DATABASE_URL, which tools/test-db-local.sh prints (see `make test-db`).
DB_URL ?= postgres://binder:binder@localhost:5432/binder?sslmode=disable

# ── Backend ──────────────────────────────────────────────────────────────────

# The lint gate is only meaningful if it reports the same issues here as in CI.
# Linter versions differ in which checks exist at all, so a stale local binary
# passes code that CI rejects, and "fixes" made against it break the gate
# instead. .golangci-version is the single source of truth; CI reads the same file.
GOLANGCI_LINT_VERSION := $(shell cat .golangci-version)

# The first-party tree, spelled out rather than `./...`. node_modules sits under
# the module root and some npm packages ship Go sources of their own, so `./...`
# picks up (and lints, and tries to test) third-party code that is not ours —
# node_modules/flatted/golang is one.
GO_ROOTS := ./internal/... ./pkg/... ./cmd/...

# internal/, pkg/ and cmd/ are still empty skeletons. Both golangci-lint and
# ginkgo treat "no packages at all" as an error, so the Go lanes would fail the
# gate on a tree that has nothing wrong with it. They skip instead, loudly, and
# start doing real work the moment the first package lands — nothing to remove
# later.
GO_SKIP_IF_EMPTY = if [ -z "$$(go list $(GO_ROOTS) 2>/dev/null)" ]; then \
	  echo "$@: no Go packages in the tree yet (internal/, pkg/, cmd/ are skeletons) — nothing to do."; \
	  exit 0; \
	fi

# The store suites need a database. Docker is the normal answer; where there is
# none (agent containers, CI runners without a container runtime) this boots a
# system Postgres and exports TEST_DATABASE_URL.
ENSURE_TEST_DB = if [ -z "$$TEST_DATABASE_URL" ]; then \
	  echo "  (no TEST_DATABASE_URL; booting a local Postgres via tools/test-db-local.sh)"; \
	  eval "$$(tools/test-db-local.sh)"; \
	fi

.PHONY: check-golangci-version
check-golangci-version: ## -> Fails unless the local golangci-lint matches .golangci-version
	@have="v$$(golangci-lint version --short 2>/dev/null)"; \
	if [ "$$have" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint is $$have, but this repo pins $(GOLANGCI_LINT_VERSION) (see .golangci-version)."; \
		echo "CI runs the pinned version, so a mismatch here reports different issues than the gate."; \
		echo "Run 'make bootstrap', then put \$$(go env GOPATH)/bin FIRST on PATH."; \
		exit 1; \
	fi

.PHONY: lint
lint: check-golangci-version ## -> Go lint (golangci-lint), pinned to .golangci-version
	@$(GO_SKIP_IF_EMPTY); \
	golangci-lint run $(GO_ROOTS)

PKGS ?= $(GO_ROOTS)

.PHONY: test
test: ## -> Go tests (Ginkgo, -race); boots a local Postgres for the store suites when TEST_DATABASE_URL is unset
	@$(GO_SKIP_IF_EMPTY); \
	$(ENSURE_TEST_DB); \
	ginkgo run -race --trace -covermode atomic $(PKGS)

# ── Agent fast lane ──────────────────────────────────────────────────────────
# `make check` is the merge gate and stays what CI runs. The targets below are
# the inner loop an agent runs dozens of times per task: the same code, scoped
# to what was touched and stripped of the flags that cost the most wall clock.
# Always finish with `make check-changed`. The fast lane is for iterating.

.PHONY: test-fast
test-fast: ## -> Inner-loop Go tests: PKGS-scoped, no -race, no coverage
	@$(GO_SKIP_IF_EMPTY); \
	$(ENSURE_TEST_DB); \
	go test -count=1 $(PKGS)

.PHONY: lint-fast
lint-fast: check-golangci-version ## -> Inner-loop Go lint, PKGS-scoped (`make lint` is the gate)
	@$(GO_SKIP_IF_EMPTY); \
	golangci-lint run $(PKGS)

# Change-scoped lanes. tools/changed-go-pkgs.sh resolves the packages whose
# files changed AND every first-party package that imports one of them — a
# change breaks its callers more often than itself. BASE=<ref> compares against
# something other than the merge-base with origin/main.
.PHONY: test-changed
test-changed: ## -> Go tests for changed packages + their importers (BASE=<ref>)
	@pkgs="$$(tools/changed-go-pkgs.sh $(BASE))"; \
	if [ -z "$$pkgs" ]; then echo "test-changed: no first-party Go package changed; nothing to run."; exit 0; fi; \
	echo "test-changed: $$(echo "$$pkgs" | wc -l | tr -d ' ') package(s)"; \
	$(ENSURE_TEST_DB); \
	go test -count=1 $$pkgs

.PHONY: lint-changed
lint-changed: check-golangci-version ## -> Go lint for changed packages + their importers (BASE=<ref>)
	@pkgs="$$(tools/changed-go-pkgs.sh $(BASE))"; \
	if [ -z "$$pkgs" ]; then echo "lint-changed: no first-party Go package changed; nothing to lint."; exit 0; fi; \
	echo "lint-changed: $$(echo "$$pkgs" | wc -l | tr -d ' ') package(s)"; \
	golangci-lint run $$pkgs

# ── Mobile (apps/mobile) ─────────────────────────────────────────────────────

.PHONY: lint-mobile
lint-mobile: ## -> ESLint for apps/mobile
	cd apps/mobile && npm run lint

.PHONY: typecheck-mobile
typecheck-mobile: ## -> tsc --noEmit for apps/mobile
	cd apps/mobile && npm run typecheck

.PHONY: test-mobile
test-mobile: ## -> Jest for apps/mobile
	cd apps/mobile && npm run test:run

.PHONY: dead-code-mobile
dead-code-mobile: ## -> knip for apps/mobile (unused files, exports, deps)
	cd apps/mobile && npm run knip

# Catches native-module / Expo SDK drift that only shows up in a real build.
# NOT part of the gate: it queries api.expo.dev, which this project's agent
# containers cannot reach (the egress proxy answers Forbidden), so a gate that
# included it could never pass where the waves actually run. Run it from a
# machine with egress when you change a native dependency.
.PHONY: expo-check
expo-check: ## -> Expo dependency drift (needs api.expo.dev; not part of `make check`)
	cd apps/mobile && npx expo install --check

# ESLint takes the changed files directly; jest scopes itself from git and pulls
# in the tests related to those files. BASE=<ref> as above.
MOBILE_BASE = $(if $(BASE),$(BASE),origin/main)

.PHONY: lint-mobile-changed
lint-mobile-changed: ## -> ESLint over changed apps/mobile files only (BASE=<ref>)
	@files="$$(tools/changed-files.sh apps/mobile '\.(ts|tsx|js|cjs)$$' $(BASE))"; \
	if [ -z "$$files" ]; then echo "lint-mobile-changed: no apps/mobile source changed."; exit 0; fi; \
	echo "lint-mobile-changed: $$(echo "$$files" | wc -l | tr -d ' ') file(s)"; \
	rel="$$(echo "$$files" | sed 's|^apps/mobile/||' | tr '\n' ' ')"; \
	cd apps/mobile && npx eslint --max-warnings 0 $$rel

.PHONY: test-mobile-changed
test-mobile-changed: ## -> Jest for apps/mobile tests related to the change (BASE=<ref>)
	cd apps/mobile && npx jest --passWithNoTests --changedSince $(MOBILE_BASE)

# ── Shared workspaces (packages/*) ───────────────────────────────────────────

# packages/types is the first workspace; it arrived already inside this loop,
# which is what the loop was here for.
#
# The `: ;` closing the loop body is load-bearing: each step is a `guard && {...}`
# whose guard exits 1 when the package has no such script, and without it that 1
# becomes the loop's — and the recipe's — exit status.
.PHONY: gate-packages
gate-packages: ## -> Lint + tests + knip for every shared workspace (packages/*)
	@if [ ! -d packages ] || [ -z "$$(ls -A packages 2>/dev/null)" ]; then \
	  echo "gate-packages: packages/ has no workspaces yet — nothing to run."; exit 0; fi; \
	for d in packages/*/; do \
	  n=$$(node -p "require('./$$d/package.json').name" 2>/dev/null) || continue; \
	  node -e "process.exit(require('./$$d/package.json').scripts?.lint?0:1)" 2>/dev/null \
	    && { echo "--- $$n lint"; npm -w $$n run lint || exit 1; }; \
	  node -e "process.exit(require('./$$d/package.json').scripts?.['test:run']?0:1)" 2>/dev/null \
	    && { echo "--- $$n test"; npm -w $$n run test:run || exit 1; }; \
	  node -e "process.exit(require('./$$d/package.json').scripts?.typecheck?0:1)" 2>/dev/null \
	    && { echo "--- $$n typecheck"; npm -w $$n run typecheck || exit 1; }; \
	  node -e "process.exit(require('./$$d/package.json').scripts?.knip?0:1)" 2>/dev/null \
	    && { echo "--- $$n knip"; npm -w $$n run knip || exit 1; }; \
	  : ; \
	done

# ── API contract ─────────────────────────────────────────────────────────────

# The document is rendered by the server binary itself, from the same route
# registration it serves, so it cannot describe an API this backend does not
# have. It needs no running server, no database and no configuration — which is
# what lets the gate below run on a CI runner that has none of them
# (.claude/rules/001-architecture.md step 8).
.PHONY: gen-spec
gen-spec: ## -> Regenerate packages/types/openapi.json from the Go source (no server needed)
	go run ./cmd/binderd -openapi > packages/types/openapi.json

.PHONY: check-spec
check-spec: gen-spec ## -> Fail if openapi.json has drifted from the Go routes
	@git diff --exit-code packages/types/openapi.json \
	  || { echo "ERROR: packages/types/openapi.json is out of date. Run 'make gen-spec' and commit the result."; exit 1; }

.PHONY: check-types
check-types: ## -> Fail if packages/types/src/api.ts has drifted from openapi.json
	npm -w @binder/types run generate
	@git diff --exit-code packages/types/src/api.ts \
	  || { echo "ERROR: packages/types/src/api.ts is out of date. Run 'make gen-spec check-types' and commit the result."; exit 1; }

# NOT parallel-safe, and must never be given -j: check-spec rewrites
# packages/types/openapi.json while check-types reads it to regenerate api.ts.
.PHONY: check-contract
check-contract: ## -> The app<->backend contract gate: Go routes -> openapi.json -> api.ts
	@$(MAKE) check-spec
	@$(MAKE) check-types

# ── Database ─────────────────────────────────────────────────────────────────

.PHONY: test-db
test-db: ## -> Print the export lines that point the suites at a local Postgres (eval them)
	@tools/test-db-local.sh

.PHONY: db-up
db-up: ## -> Start the development Postgres from docker-compose.yml
	docker compose up -d postgres

.PHONY: migrate
migrate: ## -> Apply db/migrations to the development database (DB_URL=<url>)
	tools/migrate.sh "$(DB_URL)"

.PHONY: migrate-test
migrate-test: ## -> Apply db/migrations to the test database (boots one if TEST_DATABASE_URL is unset)
	@$(ENSURE_TEST_DB); \
	tools/migrate.sh "$$TEST_DATABASE_URL"

# ── Gates ────────────────────────────────────────────────────────────────────

.PHONY: gate-go gate-mobile
# migrate-test is in the gate because CI runs it before the suites: the store
# specs test SQL against a real schema, so "the migrations apply" is part of
# what green means, and running it in only one of the two places is the drift
# check-ci-parity exists to stop.
gate-go: lint migrate-test test
gate-mobile: lint-mobile typecheck-mobile test-mobile dead-code-mobile

# The everyday inner loop: the scoped lanes for whatever layers changed, run at
# once. Advisory — it is `check-changed` that decides a task is done.
.PHONY: verify
verify: ## -> Fast scoped lint+test for what you changed, in parallel (advisory)
	@scopes="$$(tools/changed-scopes.sh $(BASE))"; \
	if [ -z "$$scopes" ]; then echo "verify: no Go or app source changed."; exit 0; fi; \
	targets=""; \
	for s in $$scopes; do \
	  case $$s in \
	    go)     targets="$$targets test-changed lint-changed";; \
	    mobile) targets="$$targets test-mobile-changed lint-mobile-changed";; \
	  esac; \
	done; \
	echo "verify: $$scopes"; \
	$(MAKE) -j2 $$targets

# The one to run before marking a task done. tools/changed-scopes.sh says which
# layers the change reaches, and only those gates run. Anything under packages/
# selects the mobile app too, because a shared-workspace change reaches it.
# Each gate is the full, unmodified gate for its layer; the scoping is at the
# layer boundary, never inside it.
.PHONY: check-changed
check-changed: ## -> Full gate for the layers this change touched, in parallel (BASE=<ref>)
	@scopes="$$(tools/changed-scopes.sh $(BASE))"; \
	if [ -z "$$scopes" ]; then echo "check-changed: no Go or app source changed; running the shared checks only."; \
	else \
	  targets=""; \
	  for s in $$scopes; do \
	    case $$s in \
	      go)       targets="$$targets gate-go";; \
	      mobile)   targets="$$targets gate-mobile";; \
	      packages) targets="$$targets gate-packages";; \
	    esac; \
	  done; \
	  echo "check-changed: $$scopes -> $(MAKE)$$targets"; \
	  $(MAKE) -j3 $$targets || exit 1; \
	fi; \
	if echo "$$scopes" | grep -qE '^(go|packages)$$'; then $(MAKE) check-contract || exit 1; fi; \
	$(MAKE) check-ci-parity

.PHONY: check
check: gate-go gate-mobile gate-packages check-contract check-ci-parity ## -> Full quality gate: every layer, serial (what CI runs)

.PHONY: check-ci-parity
check-ci-parity: ## -> Fail if the local gate and the CI workflow have drifted apart
	@bash tools/check-ci-parity.sh

# ── Setup ────────────────────────────────────────────────────────────────────

# Installs through the module proxy, never through golangci-lint's install.sh:
# that script fetches from raw.githubusercontent.com, which this project's
# containers answer with 403. proxy.golang.org is reachable, so `go install` is
# the one path that works on a laptop and in an agent container alike.
.PHONY: bootstrap
bootstrap: ## -> Fresh container -> every gate runnable: npm deps + pinned golangci-lint + ginkgo
	npm ci
	$(MAKE) install-go-tools
	$(MAKE) warm
	@echo
	@echo "bootstrap done. The image ships an older golangci-lint in /usr/local/bin,"
	@echo "so PREPEND the Go bin dir in the same shell as the gate — appending still"
	@echo "runs the old one and reports version-drift hits the pinned version does not:"
	@echo '    export PATH="$$(go env GOPATH)/bin:$$PATH"'

GINKGO_VERSION ?= v2.26.0

# CI installs the Go tools with this same target, so the pinned versions cannot
# drift between the workflow and the Makefile — there is only one copy.
.PHONY: install-go-tools
install-go-tools: ## -> Install the pinned Go tools (golangci-lint, ginkgo) into the Go bin dir
	GOFLAGS=-mod=mod go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOFLAGS=-mod=mod go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

# Every cache here is cold in a fresh container, and one of them stays cold even
# after a session of fast-lane runs: `-race` keeps a build cache entirely
# separate from the normal one, so `test-fast` never warms what `make check`
# needs. Paying these costs here, in one labelled step, beats paying them
# scattered through a task where they look like the agent stalling. Warming
# never fails the build: a lint or type error is the gate's to report, not this
# step's, and the cache is populated either way.
.PHONY: warm warm-go warm-mobile
warm-go:
	@if [ -n "$$(go list $(GO_ROOTS) 2>/dev/null)" ]; then \
	  go build $(GO_ROOTS) || true; \
	  go build -race $(GO_ROOTS) || true; \
	fi

warm-mobile:
	-cd apps/mobile && npx tsc --noEmit --incremental --tsBuildInfoFile .tsbuildinfo

warm: ## -> Prime the build caches (Go, Go -race, tsc) so the first real run is warm
	@echo "warm: priming Go, Go -race and tsc caches"
	@$(MAKE) -j2 warm-go warm-mobile
	@echo "warm: done"

.PHONY: help
help: ## -> This map
	@echo ""
	@echo "  The four that matter"
	@echo "    make bootstrap       once, in a fresh container (npm deps + pinned linters)"
	@echo "    make verify          while you work: scoped lint+test for what changed"
	@echo "    make check-changed   before done: the FULL gate, for the layers you touched"
	@echo "    make check           the whole gate, every layer, serial"
	@echo ""
	@echo "  verify and check-changed both scope by layer: go, mobile, packages."
	@echo "  Anything under packages/ counts as the mobile app too. BASE=<ref>"
	@echo "  compares against something other than the merge-base with origin/main."
	@echo ""
	@echo "  Everything else"
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | grep -vE '^(bootstrap|verify|check-changed|check|help):' \
	  | sort | awk 'BEGIN{FS=":.*?## "}{printf "    %-24s %s\n", $$1, $$2}'
	@echo ""
