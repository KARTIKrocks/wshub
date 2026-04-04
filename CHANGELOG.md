# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.3] - 2026-04-04

### Changed

- **Zero-allocation exclude set for small client lists** — `broadcastExceptWithType` and `broadcastToRoomExceptWithType` no longer allocate a `map[*Client]struct{}` when excluding ≤4 clients; a linear pointer scan via `slices.Contains` is used instead (matches the existing `buildExcludeSet` pattern for ID-based exclusions, threshold 4); for the dominant single-sender echo-suppression case this eliminates a heap allocation per broadcast call
- `broadcastExceptClients` signature updated to accept `except []*Client` + `excludeSet map[*Client]struct{}` (mirrors `isExcludedByID` calling convention); inline exclusion loop in `broadcastToRoomExceptWithType` replaced with a single `broadcastExceptClients` call, removing duplicated parallel/sequential branching
- Added `buildClientExcludeSet` and `isExcludedClient` helpers (counterparts to `buildExcludeSet`/`isExcludedByID`)

### Fixed

- **Proper WebSocket close frame on post-upgrade connection rejection** — when `UpgradeConnection` rejects a connection after the WebSocket upgrade (per-user limit, hub shutdown during registration), it now sends a close frame before closing; connection-limit rejections use code **1013 Try Again Later**, hub-shutdown closures use code **1001 Going Away**; previously clients saw an abrupt TCP close with no WebSocket close frame

## [1.2.2] - 2026-04-03

### Changed

- **Lock-free hub broadcast snapshots** — hub-level `clientsSnapshot` now stores a `hubSnapshot` struct containing both a map (`set`) and a pre-built slice (`slice`), computed once in `updateClientsSnapshot()`; `parallelSend` and `sendWithContext` use the pre-built slice directly, eliminating the per-broadcast `snapshotToSlice` allocation; at 50K clients, parallel broadcast memory drops from 401 KB/op to ~0 B/op (−99.99%) with ~4–16% lower latency
- `broadcastExceptClients` now accepts `[]*Client` instead of `map[*Client]struct{}`, iterating the pre-built slice in both parallel and sequential paths
- `Clients()` returns a copy of the pre-built slice instead of converting from map
- Removed `snapshotToSlice` helper (no longer needed)

## [1.2.1] - 2026-04-03

### Fixed

- **`handleUnregister` now drains the send buffer** — when a client disconnects abnormally (readPump exits), any messages buffered in `client.send` are now drained immediately rather than waiting for GC; previously these were silently leaked until the client struct was collected
- **`sendMu` always released in DropOldest path** — extracted `trySendDropOldest` helper uses `defer sendMu.Unlock()` so the per-client mutex is correctly released even if a send-on-closed-channel panic propagates through the recover guard in `trySendErr`

## [1.2.0] - 2026-04-02

### Added

- **Graceful drain** — `Hub.Drain(ctx)` stops accepting new connections (HTTP 503) while letting existing connections finish in-flight messages; designed for zero-downtime rolling deploys (Kubernetes `preStop`, SIGTERM handlers)
- `WithDrainTimeout(duration)` option to configure idle connection reaping during drain (default: 30s); connections whose send buffers have been empty for this duration are proactively closed with `CloseGoingAway` (1001); set to 0 to disable the reaper
- `HubState` enum (`StateRunning`, `StateDraining`, `StateStopped`) with `String()` method for hub lifecycle inspection
- `Hub.State()`, `Hub.IsRunning()`, `Hub.IsDraining()` methods for health/readiness probes
- `ErrHubDraining` and `ErrHubStopped` sentinel errors returned by `UpgradeConnection` when the hub is not in the running state
- `UpgradeConnection` now rejects connections during drain/stopped states with HTTP 503 before running `BeforeConnect` hooks
- Idle connection drain reaper (`runDrainReaper`) that tracks per-client idle time and closes idle connections after the configured timeout
- `Shutdown` now unblocks any pending `Drain()` call and transitions state to `StateStopped`
- Documentation in `doc.go` for the graceful draining workflow with code example
- Comprehensive test suite for drain: state transitions, no-client drain, wait-for-clients, reject-during-drain, context timeout, idle reaper, active-client survival, double-drain, drain-then-shutdown, shutdown-then-drain, and drain-timeout-zero

### Changed

- `Shutdown` now sets `StateStopped` and closes `drainDone` before cancelling the context, ensuring correct state transitions
- `handleUnregister` signals drain completion when the last client disconnects during drain

## [1.1.3] - 2026-03-29

### Added

- **Pre-serialized JSON API** — `NewRawJSONMessage(data)`, `Hub.BroadcastRawJSON(data)`, and `Client.SendRawJSON(data)` accept already-marshaled `[]byte` JSON, skipping serialization entirely; ideal for marshal-once fan-out patterns (0 allocs, ~35 ns vs ~1,000 ns for `NewJSONMessage`)

## [1.1.2] - 2026-03-26

### Changed

- **Worker pool for parallel broadcast** — `parallelSend` and `sendWithContext` now dispatch batches to a persistent pool of goroutines instead of spawning new goroutines per broadcast call; at 50K clients with batch size 100, allocations drop from 102/op to 2/op (goroutine churn eliminated)
- Worker pool is lazily initialized via `sync.Once` and cleanly shut down during `Hub.Shutdown`
- Pool shutdown is safe against double-close and post-shutdown broadcasts (graceful fallback to sequential send)

### Added

- `WithParallelBroadcastWorkers(n int)` option to configure the number of persistent worker goroutines (default: `runtime.NumCPU()`)

## [1.1.1] - 2026-03-26

### Changed

- **Lock-free room broadcast snapshots** — `Room` now stores an `atomic.Value` snapshot (`[]*Client`) rebuilt on join/leave, eliminating per-broadcast slice allocations in `BroadcastToRoom`, `BroadcastToRoomExcept`, `BroadcastToRoomWithContext`, and `RoomClients`; at 1M clients, room broadcast allocations drop from 8 MB/op to 0 B/op
- `RoomCount` now reads the atomic snapshot length instead of acquiring `room.mu`
- `broadcastToRoomExceptByIDs` (adapter receive path) uses the lock-free snapshot instead of iterating under `room.mu.RLock`
- Presence publisher (`presence.go`) uses lock-free snapshot for room counts instead of acquiring per-room `RLock`

### Added

- `loadRoomSnapshot` and `rebuildRoomSnapshot` helpers for room-level atomic snapshots
- Tests for room snapshot correctness: join, leave, disconnect, caller isolation, and concurrent broadcast-with-mutation

## [1.1.0] - 2026-03-25

### Added

- **Multi-node adapter pattern** (`Adapter` interface) for horizontal scaling via shared message bus
- Redis adapter (`adapter/redis`) and NATS adapter (`adapter/nats`) as separate Go modules
- `AdapterMessage` wire format with constants for broadcast, room, user, and client operations
- **Presence gossip** (`WithPresence`) for cluster-wide client and room counts
- `GlobalClientCount()` and `GlobalRoomCount(room)` methods for cross-node totals with automatic stale-node eviction
- **Backpressure control** with `DropPolicy` (`DropNewest`, `DropOldest`) configurable via `WithDropPolicy`
- `OnSendDropped` hook fired when a message is dropped due to a full send buffer
- `WithAdapter`, `WithNodeID`, `WithPresence`, `WithHookTimeout`, `WithDropPolicy`, `WithoutHandlerLatency` options
- `WithUserID` upgrade option for atomic user ID assignment during `UpgradeConnection`
- `SendMessageWithContext` method for type-aware sends with context support
- `NodeID()` accessor on Hub
- `UpgradeOption` type for per-connection configuration
- Config validation (`validateConfig`) with warnings for very small buffer sizes
- `isChanSendPanic` helper to safely recover from sends on closed channels
- Benchmarks for `SendToUser`, `SendToClient`, `GlobalClientCount`, `GlobalRoomCount`
- Example tests for drop policy, node ID, global counts, handler latency, hook timeout
- Tests for adapter, presence, backpressure, and expanded coverage suite
- `done` channel on Client for clean writePump shutdown on unregister

### Changed

- **Registration is now synchronous** — `UpgradeConnection` blocks until the Run goroutine confirms acceptance, eliminating TOCTOU races on connection limits
- `register` channel replaced with `registrationRequest`/`registrationResult` for synchronous handshake (buffered to 64)
- **Rate limiter switched from fixed-window to token-bucket algorithm** for smoother throttling
- **`BeforeDisconnect` hook now runs with a timeout** (default 5s, configurable via `WithHookTimeout`) to avoid blocking the Run loop
- **Disconnect ordering**: secondary indexes (user index, rooms) are cleaned up before removing from primary client map; room leave hooks now fire on disconnect
- `removeClientFromAllRooms` replaced by `removeClientFromAllRoomsWithHooks` — fires `BeforeRoomLeave`/`AfterRoomLeave` on disconnect
- `SetUserID` race fix — `setClientUserID` performs limit check and index update atomically under `userIndexMu`
- `addToUserIndex` now enforces `MaxConnectionsPerUser` and returns an error
- `updateClientsSnapshot` no longer acquires `RLock` (runs exclusively in the single-threaded Run goroutine)
- `loadSnapshot` helper with safe type assertion (replaces raw atomic.Value loads)
- `CloseWithCode` now closes the send channel to signal writePump (deferred close frame) instead of writing directly
- `MiddlewareChain.Execute` uses double-checked locking for thread-safe auto-build
- `MetricsMiddleware` now records only latency and errors (message count/size tracked by readPump) to avoid double-counting
- `DebugMetrics` latency fields protected by dedicated mutex instead of atomics; `errors` map uses `RWMutex`
- `AllowSameOrigin` uses `url.Parse` for correct port handling
- `applyConfigDefaults` auto-corrects `PingPeriod >= PongWait` to 90% of PongWait
- `DefaultLimits()` simplified to zero-value struct
- Hub `Shutdown` closes the adapter before waiting on goroutines
- `HandleHTTP` now logs upgrade errors
- Connection limit fast-path uses atomic `clientCount` to avoid locking `h.mu`
- Lock ordering documented on Hub struct (`mu → roomsMu → Room.mu → Client.mu → userIndexMu`)
- `deleteRoomIfEmpty` extracted as a helper with proper lock ordering
- Client metadata nil-safe (`SetMetadata` lazy-inits, `GetMetadata` handles nil map)
- `readPump` unregister uses select to avoid blocking when Run has exited
- `wg.Add(1)` moved to `NewHub` to prevent race between `go hub.Run()` and `hub.Shutdown()`
- Makefile `all` target now runs `fmt` first
- Updated benchmark numbers in README (improved broadcast, new targeted-send and presence tables)

### Removed

- `ErrWriteTimeout` and `ErrReadTimeout` sentinels (replaced by `ErrSendBufferFull`)
- `ErrClientAlreadyExists` sentinel (unused)
- `canAcceptUserConnection` and `canAcceptConnection` helpers (logic moved into Run goroutine)
- `Client.joinRoom` method (inlined into hub)
- Fixed-window rate limiter fields (`msgCount`, `msgWindowStart`)

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

[1.2.3]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.2.3
[1.2.2]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.2.2
[1.2.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.2.1
[1.2.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.2.0
[1.1.3]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.1.3
[1.1.2]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.1.2
[1.1.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.1.1
[1.1.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.1.0
[1.0.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.0.1
[1.0.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.0.0
[0.0.1]: https://github.com/KARTIKrocks/wshub/releases/tag/v0.0.1
