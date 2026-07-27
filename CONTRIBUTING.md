# Contributing to wshub

Thanks for your interest in contributing!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/wshub.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run checks: `make ci`
6. Push and open a pull request

## Development

### Prerequisites

- Go 1.22+
- golangci-lint v2

### Running Tests

```bash
make test          # run root-module tests with the race detector
make test-modules  # run tests for adapter/redis, adapter/nats, prometheus
make bench         # run benchmarks
make lint          # run linter on the root module
make ci            # run all checks across every module
```

This repository is multi-module: the adapters and the Prometheus collector each
have their own `go.mod`, so the root module's `./...` does not reach them. Use
`make ci` (or `make all`) to cover everything the way CI does.

### Code Style

- Follow standard Go conventions
- Run `gofmt` and `goimports` before committing
- All exported types and functions must have doc comments
- Keep test coverage high for new code

## Pull Requests

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation if the public API changes
- Ensure `make ci` passes before requesting review

## Reporting Issues

- Use GitHub Issues
- Include Go version, OS, and a minimal reproduction

## License

By contributing you agree that your contributions will be licensed under the MIT License.
