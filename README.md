# wshub

[![Go Reference](https://pkg.go.dev/badge/github.com/KARTIKrocks/wshub.svg)](https://pkg.go.dev/github.com/KARTIKrocks/wshub)
[![Go Report Card](https://goreportcard.com/badge/github.com/KARTIKrocks/wshub)](https://goreportcard.com/report/github.com/KARTIKrocks/wshub)
[![Go Version](https://img.shields.io/github/go-mod/go-version/KARTIKrocks/wshub)](go.mod)
[![CI](https://github.com/KARTIKrocks/wshub/actions/workflows/ci.yml/badge.svg)](https://github.com/KARTIKrocks/wshub/actions/workflows/ci.yml)
[![GitHub tag](https://img.shields.io/github/v/tag/KARTIKrocks/wshub)](https://github.com/KARTIKrocks/wshub/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![codecov](https://codecov.io/gh/KARTIKrocks/wshub/branch/main/graph/badge.svg)](https://codecov.io/gh/KARTIKrocks/wshub)

A production-ready, scalable WebSocket package for Go with support for rooms, broadcasting, multi-node clustering, middleware, hooks, and extensibility.

**[Documentation](https://kartikrocks.github.io/wshub/)** | **[API Reference](https://pkg.go.dev/github.com/KARTIKrocks/wshub)** | **[Changelog](CHANGELOG.md)**

## Features

- **Production-Ready**: Proper concurrency, graceful shutdown & drain, error handling
- **Horizontally Scalable**: Multi-node support via adapter pattern (Redis, NATS, or custom)
- **Pluggable**: Bring your own logger, metrics
- **Middleware System**: Chain handlers with custom logic
- **Lifecycle Hooks**: Hook into connection, message, room, and backpressure events
- **Room Support**: Group clients into rooms for targeted broadcasting
- **Metrics & Logging**: Built-in interfaces for observability; official Prometheus subpackage (`wshub/prometheus`)
- **Configurable**: Extensive configuration with builder pattern
- **Limits & Rate Limiting**: Control connections, rooms, and message rates
- **Backpressure Control**: Configurable drop policies with notification hooks
- **Write Coalescing**: Opt-in batching of text messages into single frames for reduced syscalls
- **Health Probes**: Built-in `/healthz` and `/readyz` handlers with JSON responses for Kubernetes
- **Global Counts**: Cluster-wide client and room counts via presence gossip
- **Zero Business Logic**: Pure infrastructure, bring your own logic

## Installation

Requires **Go 1.22+**.

```bash
go get github.com/KARTIKrocks/wshub
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/KARTIKrocks/wshub"
)

func main() {
    hub := wshub.NewHub(
        wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
            log.Printf("Message from %s: %s", client.ID, msg.Text())
            return client.Send(msg.Data) // echo back
        }),
    )

    go hub.Run()

    http.HandleFunc("/ws", hub.HandleHTTP())
    http.HandleFunc("/healthz", hub.HealthHandler())
    http.HandleFunc("/readyz", hub.ReadyHandler())

    srv := &http.Server{Addr: ":8080"}
    go func() {
        log.Println("Listening on :8080")
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // Two-phase shutdown for zero-downtime deploys
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    hub.Drain(ctx)    // stop new connections, let existing ones finish
    hub.Shutdown(ctx) // force-close anything remaining
    srv.Shutdown(ctx)
}
```

> **Note:** since v1.7.0 the default origin check is `AllowSameOrigin`. If your
> front-end is served from a different origin than the WebSocket endpoint,
> allowlist it with `wshub.AllowOrigins(...)` — see
> [Configuration → Origin Checking](https://kartikrocks.github.io/wshub/docs/configuration#origin-checking).

## Documentation

Full guides live at **[kartikrocks.github.io/wshub](https://kartikrocks.github.io/wshub/)**:

| Guide | Covers |
| --- | --- |
| [Getting Started](https://kartikrocks.github.io/wshub/docs/getting-started) | Install and run a minimal server |
| [Hub](https://kartikrocks.github.io/wshub/docs/hub) | Broadcasting, client lookup, drain, health probes, shutdown |
| [Client](https://kartikrocks.github.io/wshub/docs/client) | Per-connection sending, metadata, callbacks |
| [Messages](https://kartikrocks.github.io/wshub/docs/messages) | Message type, handlers, zero-alloc JSON fan-out |
| [Rooms](https://kartikrocks.github.io/wshub/docs/rooms) | Joining, room broadcasting, queries |
| [Middleware](https://kartikrocks.github.io/wshub/docs/middleware) | Built-in and custom middleware chains |
| [Router](https://kartikrocks.github.io/wshub/docs/router) | Event-based message dispatch |
| [Hooks](https://kartikrocks.github.io/wshub/docs/hooks) | Connection, message, and room lifecycle hooks |
| [Adapters](https://kartikrocks.github.io/wshub/docs/adapters) | Multi-node scaling via Redis or NATS |
| [Presence](https://kartikrocks.github.io/wshub/docs/presence) | Cluster-wide client and room counts |
| [Configuration](https://kartikrocks.github.io/wshub/docs/configuration) | Buffers, timeouts, compression, origin checking |
| [Limits](https://kartikrocks.github.io/wshub/docs/limits) | Connection, room, and rate limits |
| [Metrics](https://kartikrocks.github.io/wshub/docs/metrics) | Collector interface and the Prometheus subpackage |
| [Errors](https://kartikrocks.github.io/wshub/docs/errors) | Sentinel errors and `errors.Is` matching |

Exact type signatures are generated from source on
[pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub).

Runnable programs are in [`examples/`](examples/) — simple, chat, auth, metrics,
and multinode.

## Benchmarks

Two kinds of numbers below:

1. **In-process dispatch** (Go benchmarks with mock clients) — measures hub
   bookkeeping and channel push cost. Useful for spotting allocation
   regressions, not for predicting real throughput.
2. **End-to-end load tests** (real `httptest.Server` + `gorilla/websocket`
   dialer) — measures what an actual deployment will see.

Measured on an Intel i5-11400H @ 2.70GHz (12 cores), Go 1.26, Linux.

Run them yourself:

```bash
go test -bench=. -benchmem ./...      # in-process micro-benchmarks
make loadtest LOADTEST_ARGS="..."     # end-to-end load tests
```

### In-process dispatch (mock clients)

These measure how fast the hub iterates its snapshot and pushes to client
channels. They do **not** include TCP, writePump, or remote-client work.

#### Broadcast dispatch (zero allocations)

| Operation               | Clients   | Time    | Allocs |
| ----------------------- | --------- | ------- | ------ |
| `Broadcast`             | 100,000   | 22.0 ms | 0      |
| `Broadcast`             | 1,000,000 | 263 ms  | 0      |
| `BroadcastToRoom`       | 100,000   | 23.2 ms | 0      |
| `BroadcastToRoom`       | 1,000,000 | 260 ms  | 0      |
| `BroadcastExcept`       | 100,000   | 25.9 ms | 1      |
| `BroadcastExcept`       | 1,000,000 | 294 ms  | 1      |
| `BroadcastToRoomExcept` | 100,000   | 26.0 ms | 1      |
| `BroadcastToRoomExcept` | 1,000,000 | 277 ms  | 1      |

#### Targeted Send (O(1) at any scale, zero allocations)

| Operation      | Scale             | Time   | Allocs |
| -------------- | ----------------- | ------ | ------ |
| `SendToClient` | 100,000 clients   | 129 ns | 0      |
| `SendToClient` | 1,000,000 clients | 130 ns | 0      |
| `SendToUser`   | 100,000 users     | 198 ns | 1      |
| `SendToUser`   | 1,000,000 users   | 192 ns | 1      |

#### Global Counts — Presence (zero allocations)

| Operation           | Nodes | Time   | Allocs |
| ------------------- | ----- | ------ | ------ |
| `GlobalClientCount` | 5     | 63 ns  | 0      |
| `GlobalClientCount` | 50    | 397 ns | 0      |
| `GlobalClientCount` | 100   | 715 ns | 0      |
| `GlobalClientCount` | 500   | 4.2 μs | 0      |
| `GlobalRoomCount`   | 5     | 118 ns | 0      |
| `GlobalRoomCount`   | 50    | 823 ns | 0      |
| `GlobalRoomCount`   | 100   | 1.7 μs | 0      |
| `GlobalRoomCount`   | 500   | 9.7 μs | 0      |

#### Client & Room Lookups (zero allocations)

| Operation                   | Time    | Allocs |
| --------------------------- | ------- | ------ |
| `GetClient` (1,000 clients) | 17.7 ns | 0      |
| `ClientCount`               | 0.28 ns | 0      |
| `GetClientByUserID`         | 51.3 ns | 0      |
| `RoomExists`                | 23.6 ns | 0      |
| `RoomCount`                 | 22.1 ns | 0      |
| `GetMetadata`               | 17.0 ns | 0      |
| `SetMetadata`               | 30.6 ns | 0      |

#### Client Send

| Operation     | Time    | Allocs |
| ------------- | ------- | ------ |
| `Send` (text) | 82.9 ns | 1      |
| `SendJSON`    | 495 ns  | 5      |

#### Middleware Chain

| Mode                 | Time    | Allocs |
| -------------------- | ------- | ------ |
| Built (cached)       | 14.3 ns | 0      |
| Unbuilt (on-the-fly) | 17.0 ns | 0      |

### Real-world load tests

End-to-end timings using real WebSocket connections via `httptest.Server` and
`gorilla/websocket.Dialer`. Latency is measured by embedding a unix-nano
timestamp in the payload and computing `now - sent` on receive. Reproduce with
`make loadtest`.

#### Connect — handshake throughput

| Clients | Connect time | Rate          | Mem/conn |
| ------- | ------------ | ------------- | -------- |
| 1,000   | 122 ms       | 8,205 conn/s  | 24.4 KB  |
| 5,000   | 371 ms       | 13,486 conn/s | 20.5 KB  |
| 10,000  | 485 ms       | 20,609 conn/s | 24.4 KB  |

#### Fanout — single broadcaster, 100 msg/s for 10s, 128 B payload

| Clients | Throughput    | p50      | p95      | p99      |
| ------- | ------------- | -------- | -------- | -------- |
| 1,000   | 100,000 msg/s | 2.53 ms  | 4.83 ms  | 6.68 ms  |
| 5,000   | 497,000 msg/s | 44.04 ms | 396.9 ms | 632.6 ms |
| 10,000  | 397,284 msg/s | 3.22 s   | 6.03 s   | 6.33 s   |

> Past ~5K clients on a single node, fanout latency grows steeply — the bottleneck
> is Go scheduler pressure across `3 × clients` goroutines (readPump + writePump
>
> - handshake server), not the hub's dispatch loop. For higher per-node fanout,
>   tune `SendChannelSize`, enable `CoalesceWrites`, or scale horizontally.

#### Rooms — broadcast scoped to a room (100 msg/s, 10s)

| Clients | Rooms | Per-room p50 | p99      |
| ------- | ----- | ------------ | -------- |
| 5,000   | 100   | 11.01 ms     | 15.19 ms |
| 10,000  | 100   | 29.15 ms     | 36.05 ms |

#### Echo — per-connection round-trip (5,000 clients, 10s)

| RTT/sec | p50      | p95      | p99      |
| ------- | -------- | -------- | -------- |
| 228,380 | 19.93 ms | 35.35 ms | 72.52 ms |

> **Note on `WithParallelBroadcast`:** in real load tests, parallel dispatch is
> consistently _slower_ than the default serial path because the per-call cost
> of `trySend` (RLock + defer/recover) dominates and parallel batching can't
> overcome it. The option remains for backward compatibility but is no longer
> recommended — use the default serial broadcast.

> Always call `Build()` on your middleware chain for best performance.

### Concurrent Access (parallel goroutines)

| Operation                 | Time    | Allocs |
| ------------------------- | ------- | ------ |
| `GetClient`               | 31.0 ns | 0      |
| `ClientCount`             | 0.23 ns | 0      |
| `Metadata` (set+get)      | 76.5 ns | 0      |
| `Broadcast` (100 clients) | 5.9 μs  | 120    |

### Message Creation

| Operation           | Time    | Allocs |
| ------------------- | ------- | ------ |
| `NewMessage`        | 30.5 ns | 0      |
| `NewTextMessage`    | 32.0 ns | 0      |
| `NewBinaryMessage`  | 30.2 ns | 0      |
| `NewJSONMessage`    | 820 ns  | 9      |
| `NewRawJSONMessage` | 30.9 ns | 0      |

## Thread Safety

All Hub and Client methods are thread-safe. The package uses:

- RWMutex for client/room maps
- Separate mutexes for callbacks
- Channels for cross-goroutine communication
- WaitGroups for graceful shutdown

## License

[MIT](LICENSE)

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
