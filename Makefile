GOLANGCI_LINT_VERSION := v2.12.2
GOIMPORTS_VERSION := v0.45.0

.PHONY: all setup deps test test-v test-prometheus vet lint lint-fix fix build bench fuzz fmt cover clean ci loadtest

all: fmt vet lint test test-prometheus build

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

## Run all tests with race detector
test:
	go test -race -count=1 ./...

## Run tests with verbose output
test-v:
	go test -race -v -count=1 ./...

## Run prometheus subpackage tests
test-prometheus:
	cd prometheus && go test -race -count=1 ./...

## Format code
fmt:
	gofmt -w .
	goimports -w .

## Run go vet
vet:
	go vet ./...

## Run golangci-lint
lint: setup
	golangci-lint run ./...

## Run golangci-lint with auto-fix
lint-fix:
	golangci-lint run --fix ./...

## Fix code formatting and linting issues
fix: fmt lint-fix

## Build all packages
build:
	go build ./...

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

## CI pipeline: vet, lint, test
ci: vet lint test
