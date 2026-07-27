# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.7.0] - 2026-07-27

The adapter fixes below ship in the separately-versioned adapter modules
(`adapter/redis` v0.2.2, `adapter/nats` v0.2.2) and do not require wshub
v1.7.0 — they work against v1.6.1 as well.

### Changed

- **BREAKING (security): `DefaultConfig()` now uses `AllowSameOrigin` instead of `AllowAllOrigins`.** The previous default accepted an upgrade from any origin, leaving every server built on defaults open to cross-site WebSocket hijacking: any page on any site could open an authenticated connection using the visitor's cookies. A startup warning was not sufficient mitigation for an insecure default.

  **Who is affected:** deployments whose browser front-end is served from a different origin than the WebSocket endpoint (for example a page on `app.example.com` connecting to `ws.example.com`). Those connections are now rejected with `403` until the origin is allowlisted:

  ```go
  hub := wshub.NewHub(wshub.WithConfig(
      wshub.DefaultConfig().WithCheckOrigin(
          wshub.AllowOrigins("https://app.example.com"),
      ),
  ))
  ```

  **Who is not affected:** same-origin browser clients, and any client that sends no `Origin` header — mobile apps, CLI tools and server-to-server callers keep working unchanged, because `AllowSameOrigin` allows originless requests. To restore the old behavior explicitly, set `WithCheckOrigin(wshub.AllowAllOrigins)`.

  **Scope of the check:** `AllowSameOrigin` compares host and port, not scheme, so an `http://example.com` origin is accepted by a server reachable at `example.com` over https. This is deliberate — behind a TLS-terminating proxy `r.TLS` is nil, so a scheme comparison would reject the legitimate `https://` origins of every proxied deployment — and it matches gorilla/websocket's own same-origin check. Use `AllowOrigins`, which compares the full origin string, if you need scheme-exact matching.

### Added

- **Rejected origins are now observable.** A rejected upgrade previously surfaced only as a bare `403` from gorilla, which is indistinguishable from a proxy or routing fault. Rejections now increment the `origin_rejected` error metric and emit a warning naming the offending origin and host, along with the call needed to allow it. The client-supplied `Origin` value is truncated on a rune boundary before being logged.

- **Integration tests for the Redis and NATS adapters.** `Publish` and `Subscribe` — the entire functional core of multi-node support — previously had no test coverage; the suites only checked option setters, `Close` idempotency and interface compliance. Both adapters now have round-trip, multi-subscriber fanout, channel/subject isolation, malformed-payload recovery, teardown, context-cancellation, goroutine-leak and concurrent-publish tests. These run against in-process brokers (miniredis and an embedded `nats-server`), so they need no external services in CI. Statement coverage: redis 30.2% → 90.6%, nats 26.5% → 94.8%.

### Fixed

- **Failed handshakes could flood the logs.** Every failed upgrade emitted two unbounded log lines — one in `UpgradeConnection`, one in `HandleHTTP` — and any unauthenticated client can force that path at will, so 20 rejected handshakes produced 40 log lines. All three handshake-failure log sites (including the new rejected-origin warning) are now rate-limited to one line per minute. The `origin_rejected` and `upgrade_failed` metrics are still incremented unconditionally, so exact counts remain available.

- **The startup "accepts every origin" warning fired on correctly-restricted configurations.** `NewHub` probed the configured `CheckOrigin` with `Origin: https://attacker.example.com`, so any checker matching a real domain suffix — including the `strings.HasSuffix(origin, ".example.com")` pattern shown in the README — accepted the probe and was reported as accepting every origin. The probe now uses random hosts under the reserved `.invalid` TLD, which no legitimate allowlist can match.

- **Redis adapter: `Close()` deadlocked permanently after `Subscribe()`.** The receive goroutine closed the `PubSub` from its own `defer`, but that defer could only run once the `for msg := range sub.Channel()` loop had exited — and the loop only exits when the `PubSub` is closed. Nothing else closed it, so the goroutine parked forever on the channel receive and `Close()` blocked on `wg.Wait()` with no timeout. Every multi-node deployment using the Redis adapter would hang on graceful shutdown.

  Closing the `PubSub` now happens in a separate watcher goroutine that waits on the subscription context, so the receive loop terminates and `Close()` returns.

- **Redis adapter: cancelling the `Subscribe` context did not stop delivery.** `Subscribe`'s doc comment promised that cancelling the context stopped the subscription, but go-redis uses the context passed to `client.Subscribe` only for the initial subscribe command — the resulting `PubSub` outlives it. Messages kept being delivered to the handler after cancellation. The same watcher goroutine fixes this, making both documented teardown paths work.

- **Redis adapter: `Close()` deadlocked after `Subscribe()` was called more than once.** `Subscribe` overwrote `a.cancel` without invoking the previous one, so the earlier subscription's watcher and receive goroutines waited forever on a context nothing would cancel, and `Close()` blocked on `wg.Wait()`. The replaced subscription also kept delivering to its old handler. `Subscribe` now releases the previous subscription before installing the new one, matching the NATS adapter's existing behavior.

- **Redis adapter: `Subscribe` added to its `WaitGroup` after releasing the adapter lock.** A `Close()` starting in that window could reach `wg.Wait()` while the counter was still zero and return before either subscription goroutine had started — and adding to a `WaitGroup` from zero concurrently with `Wait` is a documented misuse. Both slots are now reserved while the lock is held.

- **Both adapters: `Subscribe` after `Close()` silently succeeded.** `Publish` returned `ErrClosed` but `Subscribe` created a live subscription on a closed adapter, resurrecting it with goroutines that `Close()` would never wait on. Both now return `ErrClosed`.

- **NATS adapter: `Close()` concurrent with `Subscribe()` left a live subscription on a closed adapter.** `conn.Subscribe` runs without the adapter lock, and its result was installed unconditionally, so a `Close()` landing in that window produced a subscription nothing would ever drain and a watcher goroutine nothing would ever release. `Subscribe` now claims a generation before subscribing and rechecks the closed/superseded state before installing, draining the subscription instead if it lost the race. Draining is `sync.Once`-guarded so the watcher and `Close` cannot both drain the same subscription — which previously made `Close` return a spurious "invalid subscription" error. `Close` now also waits for the watcher goroutine to exit before returning.

- **NATS adapter: `Subscribe` leaked a goroutine per call when torn down via `Close()`.** The context-watcher goroutine blocked on `<-ctx.Done()` alone. Callers that pass a long-lived context (`context.Background()` is the documented usage) and shut down with `Close()` never cancelled it, so the goroutine lived for the life of the process — one per `Subscribe` call. It now also selects on an internal stop channel closed by `Close`, with exactly one of the two paths performing the drain.

## [1.6.1] - 2026-07-13

### Fixed

- **Data race between `Client.CloseWithCode` and concurrent broadcasts** — `CloseWithCode` closed the client's `send` channel while producers were still sending on it (`Hub.trySendErr`'s fast path, the `DropOldest` evict/enqueue loop, and `SendMessageWithContext`), none of which hold a lock the closer takes. Closing a channel concurrently with a send on it is a data race; the `recover()` guard around the send only hid the panic that followed. Race-enabled builds under `DropOldest` could report `WARNING: DATA RACE` between `closechan` and `chansend` within a few hundred broadcast messages.

  `CloseWithCode` now signals `writePump` through the existing `done` channel and flags the close as graceful, so queued messages are still flushed and the WebSocket close frame is still sent. Nothing closes `send` any more — `handleUnregister` continues to drain it. No API change; behavior is unchanged except that the race is gone.

## [1.6.0] - 2026-06-06

### Added

- **`WithUnmarshalErrorHandler` option for the NATS and Redis adapters** — register a `func(data []byte, err error)` callback to observe messages that fail JSON unmarshaling in the subscribe path. Previously these were silently dropped; the default behavior is unchanged when no handler is set.

### Fixed

- **Redis adapter `Close()` no longer holds the mutex across `wg.Wait()`** — the lock is released before waiting on in-flight goroutines, removing a potential deadlock if a draining goroutine needs the lock.
- **NATS adapter `Subscribe` drains any existing subscription before resubscribing** — prevents a subscription/goroutine leak when `Subscribe` is called more than once.
- **Redis adapter wraps publish/marshal errors with `%w`** — `Publish` now returns `marshal message: …` / `publish to channel <name>: …` with the underlying error preserved for `errors.Is`/`errors.As`.

### Changed

- Test suite reliability: replaced `time.Sleep`-based synchronization in adapter and coverage tests with deterministic polling, converted several tests to table-driven form, and added `t.Parallel()` where safe. The client-readiness helper now waits on `len(hub.Clients())` (the lock-free broadcast snapshot) rather than `hub.ClientCount()` (the atomic counter), which can lead the snapshot during registration.
- `Makefile`: added `lint-fix` and `fix` convenience targets.
- Documentation: added doc comments to exported `prometheus.Collector` methods.

## [1.5.2] - 2026-05-11

### Fixed

- **`writePump` no longer exits early on hub context cancellation** — shutdown now wakes the writer through the normal client close path (`client.send` / `client.done`) instead of selecting directly on `h.ctx.Done()`, giving queued messages and WebSocket close frames a chance to flush before the pump exits

## [1.5.1] - 2026-04-08

### deps

- chore: update wshub dependency version to v1.5.0 across modules by @KARTIKrocks in https://github.com/KARTIKrocks/wshub/pull/24
- test: ensure hubs are ready before returning dial functions in setupPair by @KARTIKrocks in https://github.com/KARTIKrocks/wshub/pull/25
- deps: bump github.com/redis/go-redis/v9 from 9.7.3 to 9.18.0 in /adapter/redis by @dependabot[bot] in https://github.com/KARTIKrocks/wshub/pull/20
- deps: bump github.com/prometheus/client_golang from 1.20.5 to 1.23.2 in /prometheus by @dependabot[bot] in https://github.com/KARTIKrocks/wshub/pull/23
- deps: bump github.com/nats-io/nats.go from 1.39.1 to 1.50.0 in /adapter/nats by @dependabot[bot] in https://github.com/KARTIKrocks/wshub/pull/21

## [1.5.0] - 2026-04-07

### Added

- **Official Prometheus subpackage** (`github.com/KARTIKrocks/wshub/prometheus`) — drop-in `MetricsCollector` backed by `prometheus/client_golang`; exposes `connections_active`, `connections_total`, `messages_received_total`, `messages_sent_total`, `messages_dropped_total`, `message_received_bytes_total`, `message_latency_seconds`, `broadcast_duration_seconds`, `rooms_active`, `room_joins_total`, `room_leaves_total`, and `errors_total{type}`
- `WithRegistry(reg)`, `WithNamespace(ns)`, `WithLatencyBuckets(buckets)`, `WithBroadcastBuckets(buckets)` options on the Prometheus collector
- `MetricsCollector.IncrementMessagesSent(count int)` — tracks outbound message count across all write paths (single frames, coalesced batches, drain)
- `MetricsCollector.IncrementMessagesDropped()` — dedicated counter for send-buffer-full drops (replaces the generic `IncrementErrors("send_buffer_full")` call)
- `MetricsCollector.RecordBroadcastDuration(duration)` — histogram observation for local fanout time; recorded in `broadcast`, `broadcastExceptClients`, and `broadcastToRoomClients`
- `MetricsCollector.IncrementRooms()` / `DecrementRooms()` — gauge tracking active room count; incremented in `JoinRoom`, decremented in `deleteRoomIfEmpty`
- `DebugStats.TotalMessagesSent`, `TotalDropped`, `ActiveRooms`, `AvgBroadcast` — new fields in the `DebugMetrics` snapshot
- `Makefile` targets: `test-prometheus` (runs the prometheus subpackage tests), `lint-fix` (golangci-lint with `--fix`), `fix` (`fmt` + `lint-fix`); `all` target now includes `test-prometheus`
- `prealloc` linter enabled in `.golangci.yml`; `gofmt` formatter configured with `simplify: true`

### Changed

- **`MetricsCollector.IncrementMessages()` renamed to `IncrementMessagesReceived()`** — callers implementing the interface must rename their method; `DebugStats.TotalMessages` renamed to `TotalMessagesRecv`
- `notifySendDropped` now calls `IncrementMessagesDropped()` instead of `IncrementErrors("send_buffer_full")`; custom metrics implementations that mapped `IncrementErrors` to a Prometheus label should migrate to the new dedicated counter

### Fixed

- **Infinite spin in `handleUnregister` drain loop** — when `CloseWithCode` had already closed the `client.send` channel, receiving from the closed channel returned immediately with the zero value, causing the drain loop to spin forever and hang the `Run` goroutine; the drain now checks `sendChanClosed` and skips the loop entirely in that case
- **Stale room deletion in `deleteRoomIfEmpty`** — the identity check `h.rooms[roomName] == room` now guards deletion so a room that was concurrently recreated under the same name is not accidentally removed

## [1.4.0] - 2026-04-06

### Added

- **Health and readiness probe helpers** — `Hub.HealthHandler()` and `Hub.ReadyHandler()` return drop-in `http.HandlerFunc` values for Kubernetes `/healthz` and `/readyz` endpoints; both respond with a JSON body (`alive`, `ready`, `state`, `uptime_ns`, `clients`) and set the HTTP status code automatically (200 or 503)
- `Hub.Alive() bool` — true only while the `Run()` goroutine is executing; single atomic load, safe on hot paths
- `Hub.Ready() bool` — true when the hub is alive and in `StateRunning` (accepting connections)
- `Hub.Uptime() time.Duration` — elapsed time since `Run()` started; returns zero before `Run()` is called or after it exits
- `Hub.Health() HealthStatus` — point-in-time snapshot struct with `Alive`, `Ready`, `State` (string), `Uptime`, and `Clients`; all reads are lock-free atomic loads
- `ErrHubNotStarted` sentinel error returned by `UpgradeConnection` when the hub has not yet been started

### Fixed

- **`UpgradeConnection` accepted connections before `Run()` was called** — `HubState` uses `iota` so the zero value of `state` equals `StateRunning`, causing a freshly created hub to appear running before `Run()` ever executed; `UpgradeConnection` now checks `Alive()` first and returns HTTP 503 + `ErrHubNotStarted` if the `Run()` goroutine has not started

## [1.3.0] - 2026-04-04

### Added

- **Opt-in write coalescing** — `Config.CoalesceWrites` batches queued text messages into a single WebSocket frame separated by `\n`, reducing syscalls under high throughput; binary messages are always sent as individual frames; disabled by default so existing behaviour is unchanged
- `WithCoalesceWrites(bool)` builder method on `Config`
- Documentation in `doc.go` and README for the write coalescing feature

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

[1.7.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.7.0
[1.5.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.5.0
[1.4.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.4.0
[1.3.0]: https://github.com/KARTIKrocks/wshub/releases/tag/v1.3.0
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
