GOLANGCI_LINT_VERSION := v2.12.2
GOIMPORTS_VERSION := v0.45.0

# Nested modules. Each has its own go.mod, so the root module's ./... does not
# reach them — they need to be built and tested explicitly.
SUBMODULES := adapter/redis adapter/nats prometheus
EXAMPLE_MODULES := examples/multinode

.PHONY: all setup deps work test test-v test-modules test-prometheus vet vet-modules lint lint-modules lint-fix fix build build-examples bench fuzz fmt cover clean ci loadtest

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

## Download module dependencies
deps:
	go mod download

## Create go.work (gitignored) so nested modules resolve wshub from the working
## tree instead of the published version pinned in their go.mod. Mirrors what CI
## does with `go mod edit -replace`, so local runs catch the same API drift.
work:
	@test -f go.work || go work init . $(SUBMODULES) $(EXAMPLE_MODULES)

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

## Run golangci-lint with auto-fix
lint-fix:
	golangci-lint run --fix ./...

## Fix code formatting and linting issues
fix: fmt lint-fix

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

## CI pipeline: vet, lint, test across every module
ci: vet vet-modules lint lint-modules test test-modules
