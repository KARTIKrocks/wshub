# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-03-13

### Added

- `gocyclo` linter with max complexity 15 in `.golangci.yml`
- Dependabot configuration for Go modules and GitHub Actions (weekly schedule)
- GitHub issue templates for bug reports and feature requests
- Pull request template with checklist
- CodeQL security scanning workflow (push, PR, weekly schedule)
- Code coverage reporting with Codecov integration
- Coverage badge in README

### Changed

- CI workflow now restricts `permissions` to `contents: read`
- Bench job limited to `main` branch pushes only (skipped on PRs)

## [0.0.1] - 2026-02-20

### Added

- Core hub with channel-based event loop, graceful shutdown, and context support
- Client management with UUID-based IDs, per-client metadata, and user ID tracking
- Room support with per-room locks, lazy creation, and automatic cleanup
- Lock-free snapshot broadcasting with optional parallel batching for 1000+ clients
- Functional options pattern for hub configuration (`WithConfig`, `WithLogger`, `WithHooks`, etc.)
- Pluggable `Logger` interface with `NoOpLogger` default
- Pluggable `MetricsCollector` interface with `DebugMetrics` in-memory implementation
- `MiddlewareChain` with `Build()` caching and built-in `LoggingMiddleware`, `RecoveryMiddleware`, `MetricsMiddleware`
- Format-agnostic `Router` with extractor-based message dispatching
- `Config` builder with `DefaultConfig()`, buffer sizes, timeouts, compression, origin checking
- `Limits` builder with connection, room, and rate limiting controls
- `Hooks` for full lifecycle callbacks (connect, disconnect, message, room join/leave, error)
- Sentinel errors for all failure modes (`ErrConnectionClosed`, `ErrEmptyRoomName`, etc.)
- O(1) client lookup by ID and user ID indexing for multi-device support
- `BroadcastWithContext` for timeout-aware broadcasting
- Origin checking helpers: `AllowAllOrigins`, `AllowSameOrigin`, `AllowOrigins`
- Comprehensive test suite with race detector coverage
- CI via GitHub Actions (Go 1.23/1.24/1.25/1.26 matrix, lint, bench)
- golangci-lint v2 configuration
- Examples: simple echo server, chat with rooms, JWT auth, metrics endpoint
- Documentation: README, QUICKSTART, SCALABILITY, CONTRIBUTING

[1.0.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.0.0
[0.0.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v0.0.1
