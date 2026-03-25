# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-03-20

### Added

- Tests for `SendToUser`, `BroadcastBinary`, `RoomClients`, `BroadcastToRoomExcept`, parallel broadcast paths
- Tests for buffer-full scenarios (`trySend`, `SendMessage`), `BeforeConnect` hook rejection, connection limits
- Tests for `readPump` message handlers, `BeforeMessage`/`AfterMessage` hooks, message rejection
- Tests for `OnClose` callback, `OnMessage` callback, `SendJSON` error path, `SendWithContext` closed client
- Tests for room hooks (`BeforeRoomJoin`, `AfterRoomJoin`, `BeforeRoomLeave`, `AfterRoomLeave`)
- Tests for lifecycle hooks (`AfterConnect`, `BeforeDisconnect`, `AfterDisconnect`)
- Tests for `UpdateClientUserID`, `JoinRoom` (already in room, client not found, max rooms), `LeaveRoom` (not in room)
- Tests for `HandleHTTP` upgrade, `BroadcastWithContext` cancellation, `BroadcastJSON` error path
- Fuzz tests for message parsing, JSON creation, router dispatch, and middleware chain
- Example tests for `go doc` integration (hub, message, router, middleware, config, limits, metrics, hooks)
- Benchmark suite covering broadcasts, client sends, lookups, rooms, metadata, and middleware
- Codecov configuration (`codecov.yml`) with patch target 80% and project threshold 2%
- `.gitignore` for build artifacts and coverage files
- Update README docs
- `make cover` target for HTML coverage reports
- `make fuzz` target for fuzz testing
- `make build`, `make test-v`, `make clean` targets
- `make setup` with conditional tool installation

### Changed

- Pinned golangci-lint to v2.10.1 in Makefile for reproducible builds
- `make lint` now auto-installs linter via `setup` dependency
- README "JavaScript Client" section replaced with full HTML test client
- CONTRIBUTING.md: fixed clone URL to use fork, corrected Go version to 1.22+

### Removed

- `QUICKSTART.md` (content consolidated into README)

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

[1.0.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.0.1
[1.0.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.0.0
[0.0.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v0.0.1
