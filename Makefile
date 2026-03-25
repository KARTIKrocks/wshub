GOLANGCI_LINT_VERSION := v2.10.1

.PHONY: all setup deps test test-v vet lint build bench fuzz fmt cover clean ci

all: vet lint test build

## Install development tools (skips if already present)
setup:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	}
	@command -v goimports >/dev/null 2>&1 || { \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
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

## Run go vet
vet:
	go vet ./...

## Run golangci-lint
lint: setup
	golangci-lint run ./...

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

## Format code
fmt:
	gofmt -w .
	goimports -w .

## Run tests with coverage report
cover:
	go test -race ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Remove build artifacts
clean:
	rm -f coverage.out coverage.html

## CI pipeline: vet, lint, test
ci: vet lint test
