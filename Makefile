GOLANGCI_LINT_VERSION := v2.12.2
GOIMPORTS_VERSION := v0.48.0
GOVULNCHECK_VERSION := v1.6.0

# The Markdown linter. Versioned in website/package.json rather than pinned
# here, so Dependabot keeps it current along with the rest of the docs toolchain.
MARKDOWNLINT := website/node_modules/.bin/markdownlint-cli2

# Nested modules. Each has its own go.mod, so the root module's ./... does not
# reach them — they need to be built and tested explicitly.
SUBMODULES := adapter/redis adapter/nats prometheus
EXAMPLE_MODULES := examples/multinode

.PHONY: all setup deps work tidy tidy-modules tidy-check test test-v test-modules test-prometheus vet vet-modules lint lint-modules lint-docs lint-docs-fix lint-fix fix vuln vuln-modules print-govulncheck-version build build-examples bench fuzz fmt cover clean ci loadtest

all: fmt vet vet-modules lint lint-modules test test-modules build build-examples

## Install development tools (skips if already present)
setup:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	}
	@command -v goimports >/dev/null 2>&1 || { \
		echo "Installing goimports $(GOIMPORTS_VERSION)..."; \
		go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION); \
	}
	@command -v govulncheck >/dev/null 2>&1 || { \
		echo "Installing govulncheck $(GOVULNCHECK_VERSION)..."; \
		go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
	}

## Download module dependencies
deps:
	go mod download

## Create go.work (gitignored) so nested modules resolve wshub from the working
## tree instead of the published version pinned in their go.mod. Mirrors what CI
## does with `go mod edit -replace`, so local runs catch the same API drift.
work:
	@test -f go.work || go work init . $(SUBMODULES) $(EXAMPLE_MODULES)

## Tidy go.mod/go.sum for the root module
tidy:
	go mod tidy

## Tidy go.mod/go.sum for every nested module. Run this after bumping the wshub
## pin in a submodule so go.sum records the new version.
##
## Unlike the other module targets this deliberately does not depend on `work`:
## `go mod tidy` ignores the workspace, so it resolves wshub at the version each
## go.mod pins and downloads it from the proxy. That is what consumers get, and
## it is why a pin bumped to an unreleased tag fails here rather than silently
## passing against the working tree.
tidy-modules:
	@for m in $(SUBMODULES) $(EXAMPLE_MODULES); do \
		echo "==> tidy $$m"; \
		(cd $$m && go mod tidy) || exit 1; \
	done

## Fail if any go.mod/go.sum is not tidy, without leaving the change behind.
## Suitable for CI, where a stale go.sum should block the merge.
tidy-check:
	@status=$$(git status --porcelain -- go.mod go.sum $(addsuffix /go.mod,$(SUBMODULES) $(EXAMPLE_MODULES)) $(addsuffix /go.sum,$(SUBMODULES) $(EXAMPLE_MODULES))); \
	if [ -n "$$status" ]; then \
		echo "go.mod/go.sum already modified; commit or stash before running tidy-check"; \
		exit 1; \
	fi
	@$(MAKE) --no-print-directory tidy tidy-modules
	@if ! git diff --quiet -- '*go.mod' '*go.sum'; then \
		echo "go.mod/go.sum are not tidy — run 'make tidy tidy-modules' and commit:"; \
		git diff --stat -- '*go.mod' '*go.sum'; \
		git checkout -- '*go.mod' '*go.sum'; \
		exit 1; \
	fi
	@echo "all modules tidy"

## Run all tests with race detector
test:
	go test -race -count=1 ./...

## Run tests with verbose output
test-v:
	go test -race -v -count=1 ./...

## Run tests for every nested module
test-modules: work
	@for m in $(SUBMODULES); do \
		echo "==> test $$m"; \
		(cd $$m && go test -race -count=1 ./...) || exit 1; \
	done

## Run prometheus subpackage tests
test-prometheus: work
	cd prometheus && go test -race -count=1 ./...

## Format code
fmt:
	gofmt -w .
	goimports -w .

## Run go vet
vet:
	go vet ./...

## Run go vet for every nested module
vet-modules: work
	@for m in $(SUBMODULES) $(EXAMPLE_MODULES); do \
		echo "==> vet $$m"; \
		(cd $$m && go vet ./...) || exit 1; \
	done

## Run golangci-lint
lint: setup
	golangci-lint run ./...

## Run golangci-lint for every nested module
lint-modules: setup work
	@for m in $(SUBMODULES); do \
		echo "==> lint $$m"; \
		(cd $$m && golangci-lint run ./...) || exit 1; \
	done

## Scan the root module for known vulnerabilities, filtered to advisories this
## code actually reaches. Mirrors the `vuln` CI job, which gates merges.
##
## Needs network access — the advisory database is fetched on every run.
##
## Note this also scans the standard library of whichever Go toolchain you have
## installed, so it can fail locally on a green branch when your Go is a patch
## release behind the one CI pins. That is a real finding about your machine,
## not a false positive.
vuln: setup
	govulncheck ./...

## Print the pinned scanner version. CI installs govulncheck with this rather
## than hardcoding a second copy of the number, so the workflow and this file
## cannot drift apart.
print-govulncheck-version:
	@echo $(GOVULNCHECK_VERSION)

## Scan every nested module. Separate for the same reason lint-modules is: the
## root module's ./... stops at nested go.mod boundaries, so the adapters' own
## dependency trees (redis, nats) are invisible to a scan started from here.
vuln-modules: setup work
	@for m in $(SUBMODULES); do \
		echo "==> vuln $$m"; \
		(cd $$m && govulncheck ./...) || exit 1; \
	done

## Lint every Markdown file in the repo — the docs site, the README, and the
## contributor/security policies. Config and rationale live in
## .markdownlint-cli2.jsonc. Needs Node; the binary comes from website/, which
## is the only npm project here.
lint-docs: $(MARKDOWNLINT)
	@website/node_modules/.bin/markdownlint-cli2

## Lint Markdown and apply the fixes it can make automatically.
lint-docs-fix: $(MARKDOWNLINT)
	@website/node_modules/.bin/markdownlint-cli2 --fix

$(MARKDOWNLINT):
	@echo "Installing website dependencies (needed for the Markdown linter)..."
	@cd website && npm ci

## Run golangci-lint with auto-fix
lint-fix:
	golangci-lint run --fix ./...

## Fix code formatting and linting issues
fix: fmt lint-fix lint-docs-fix

## Build all packages
build:
	go build ./...

## Build example modules that live outside the root module
build-examples: work
	@for m in $(EXAMPLE_MODULES); do \
		echo "==> build $$m"; \
		(cd $$m && go build ./...) || exit 1; \
	done

## Run benchmarks
bench:
	go test -bench=. -benchmem ./...

## Run fuzz tests (default 30s per target)
fuzz:
	go test -fuzz=FuzzMessageJSON -fuzztime=30s .
	go test -fuzz=FuzzNewJSONMessage -fuzztime=30s .
	go test -fuzz=FuzzRouterDispatch -fuzztime=30s .
	go test -fuzz=FuzzMiddlewareChain -fuzztime=30s .

## Run tests with coverage report
cover:
	go test -race ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Run load tests against a real wshub server with real WebSocket connections.
## Examples:
##   make loadtest                                                                # all scenarios, 1000 clients
##   make loadtest LOADTEST_ARGS="-scenario=fanout -clients=10000"                # fanout only
##   make loadtest LOADTEST_ARGS="-scenario=fanout -clients=10000 -parallel=100"  # parallel broadcast
##   make loadtest LOADTEST_ARGS="-scenario=churn -clients=5000 -churn-rate=200"  # churn stress test
loadtest:
	go run ./cmd/loadtest/ $(LOADTEST_ARGS)

## Remove build artifacts
clean:
	rm -f coverage.out coverage.html

## Everything the `ci` merge gate checks, in one target: vet, lint, test and
## vulnerability-scan every module, plus Markdown across the repo.
##
## Heavier than `make all` on purpose — it needs the network for the advisory
## database and Node for the Markdown linter. Use `all` in the edit loop and
## this before opening a pull request.
ci: vet vet-modules lint lint-modules test test-modules vuln vuln-modules lint-docs
