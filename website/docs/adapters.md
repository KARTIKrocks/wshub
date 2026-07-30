---
id: adapters
title: Multi-Node Adapters
sidebar_label: Adapters
description: Scale horizontally by relaying broadcasts and targeted sends across hub instances through a shared message bus.
---

# Multi-Node Adapters

Scale horizontally by relaying broadcasts and targeted sends across multiple hub
instances through a shared message bus.

```go
import "github.com/KARTIKrocks/wshub"
```

- Pluggable `Adapter` interface for cross-node communication
- Built-in Redis and NATS adapter implementations
- Automatic deduplication via node IDs
- All broadcast and send methods relay transparently
- Local delivery is never blocked by adapter failures
- Observable unmarshal failures on the subscribe path
- Deterministic teardown — `Close` waits for subscriber goroutines

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Adapter).

## Adapter Interface

Implement the `Adapter` interface to integrate with any message bus:

```go
type Adapter interface {
    // Publish sends a message to all other nodes.
    Publish(ctx context.Context, msg AdapterMessage) error

    // Subscribe begins receiving messages from other nodes.
    // Must not block — spawn goroutines internally.
    Subscribe(ctx context.Context, handler func(AdapterMessage)) error

    // Close shuts down the adapter, releasing all resources.
    Close() error
}
```

## Adapter Message

The wire format used for inter-node communication:

| Field | Description |
| --- | --- |
| `NodeID` | Originating hub node (used for deduplication) |
| `Type` | Operation type (broadcast, room, user, client, presence) |
| `Room` | Target room name (for room-scoped operations) |
| `UserID` | Target user (for `SendToUser`) |
| `ClientID` | Target client (for `SendToClient`) |
| `ExceptClientIDs` | Client IDs to exclude from delivery |
| `MsgType` | WebSocket message type (text or binary) |
| `Data` | Raw message payload |

Supported operation types:

| Constant | Description |
| --- | --- |
| `AdapterBroadcast` | Broadcast to all clients |
| `AdapterBroadcastExcept` | Broadcast excluding specific clients |
| `AdapterRoom` | Broadcast to a room |
| `AdapterRoomExcept` | Broadcast to a room excluding specific clients |
| `AdapterUser` | Send to all connections of a user |
| `AdapterClient` | Send to a specific client |
| `AdapterPresence` | Presence heartbeat |

## Redis Adapter

Uses Redis Pub/Sub for cross-node communication. Install the adapter module:

```bash
go get github.com/KARTIKrocks/wshub/adapter/redis
```

```go
import (
    "github.com/KARTIKrocks/wshub"
    wshubredis "github.com/KARTIKrocks/wshub/adapter/redis"
    goredis "github.com/redis/go-redis/v9"
)

rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})

adapter := wshubredis.New(rdb,
    wshubredis.WithChannel("myapp:wshub"), // default: "wshub:messages"
)

hub := wshub.NewHub(
    wshub.WithAdapter(adapter),
    wshub.WithNodeID("node-1"), // optional: stable ID for debugging
)
go hub.Run()
```

## NATS Adapter

Uses NATS core Pub/Sub for lower-latency cross-node communication. Install the
adapter module:

```bash
go get github.com/KARTIKrocks/wshub/adapter/nats
```

```go
import (
    "github.com/KARTIKrocks/wshub"
    wshubnats "github.com/KARTIKrocks/wshub/adapter/nats"
    gonats "github.com/nats-io/nats.go"
)

nc, _ := gonats.Connect("nats://localhost:4222")

adapter := wshubnats.New(nc,
    wshubnats.WithSubject("myapp.wshub"), // default: "wshub.messages"
)

hub := wshub.NewHub(
    wshub.WithAdapter(adapter),
    wshub.WithNodeID("node-1"),
)
go hub.Run()
```

## Adapter Options

| Option | Description |
| --- | --- |
| `WithChannel(name)` | Redis only — Pub/Sub channel. Default `wshub:messages` |
| `WithSubject(name)` | NATS only — subject. Default `wshub.messages` |
| `WithUnmarshalErrorHandler(fn)` | Observe messages that fail JSON unmarshaling in the subscribe path |

Added in v1.6.0. A malformed payload on the bus — a stray publisher, a partial
write, a version mismatch between nodes — is dropped by the subscribe path.
Without a handler that happens silently, so a node quietly stops relaying with
nothing in the logs. Both adapters accept the same option:

```go
adapter := wshubredis.New(rdb,
    wshubredis.WithUnmarshalErrorHandler(func(data []byte, err error) {
        log.Printf("wshub: dropping malformed adapter message: %v (%d bytes)", err, len(data))
    }),
)

// Identical on the NATS adapter:
adapter := wshubnats.New(nc,
    wshubnats.WithUnmarshalErrorHandler(func(data []byte, err error) {
        log.Printf("wshub: dropping malformed adapter message: %v (%d bytes)", err, len(data))
    }),
)
```

## How It Works

When an adapter is configured, every public broadcast and send method delivers
locally first, then publishes to the adapter so other nodes can relay the
message to their clients. Messages originating from the current node are
automatically deduplicated via the node ID.

```go
// These all work transparently across nodes:
hub.Broadcast([]byte("hello everyone"))          // all nodes
hub.BroadcastToRoom("chat", []byte("hi room"))   // room members on all nodes
hub.SendToUser("user-123", []byte("hi"))          // user's connections on all nodes
hub.SendToClient(clientID, []byte("hi"))          // finds client across nodes

// Shutdown closes the adapter before waiting on goroutines
hub.Shutdown(ctx)
```

## Lifecycle

The bundled adapters guarantee the following teardown behaviour as of
`adapter/redis v0.2.2` and `adapter/nats v0.2.2`.

:::warning

Earlier releases could deadlock in `Close` after a `Subscribe`, so upgrade both
adapters alongside wshub v1.7.0.

:::

- `Close` waits for the subscriber goroutine to finish, so no delivery is in
  flight once it returns.
- `Close` is idempotent — calling it twice returns `nil` the second time.
  `Hub.Shutdown` already closes a configured adapter, so an explicit call is
  only needed for an adapter you own outside a hub.
- After `Close`, both `Publish` and `Subscribe` return `ErrClosed` rather than
  silently doing nothing.
- Calling `Subscribe` again replaces the previous subscription and releases its
  goroutine.
- Cancelling the context passed to `Subscribe` stops delivery.

```go
// A hub-owned adapter needs no explicit Close — Shutdown does it:
hub.Shutdown(ctx)

// A standalone adapter is yours to close, before the underlying
// client goes away:
adapter := wshubredis.New(rdb)
defer rdb.Close()
defer adapter.Close() // runs first: waits for the subscriber goroutine
```
