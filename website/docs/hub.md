---
id: hub
title: Hub
description: The central connection manager that handles all WebSocket clients, rooms, and message routing.
---

# Hub

The central connection manager that handles all WebSocket clients, rooms, and
message routing.

```go
import "github.com/KARTIKrocks/wshub"
```

- Manages all connected WebSocket clients
- Broadcasts to all clients, specific clients, or rooms
- O(1) client and user lookups via hash maps
- Snapshot-based lock-free broadcasting
- Optional parallel broadcasting for 1000+ clients
- Multi-node support via pluggable adapters
- Configurable backpressure with drop policies
- Graceful shutdown with context support
- Built-in health and readiness handlers for Kubernetes probes

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Hub).

## Creating a Hub

Create a hub with functional options and start the run loop:

```go
hub := wshub.NewHub(
    wshub.WithConfig(config),
    wshub.WithLogger(logger),
    wshub.WithMetrics(metrics),
    wshub.WithLimits(limits),
    wshub.WithHooks(hooks),
    wshub.WithMessageHandler(handler),
    wshub.WithParallelBroadcast(100), // batch size for parallel broadcast
    wshub.WithAdapter(adapter),       // multi-node support
    wshub.WithPresence(5*time.Second),// cluster-wide counts
    wshub.WithDropPolicy(wshub.DropOldest), // backpressure control
)

// Start the hub run loop (required)
go hub.Run()

// Register as HTTP handler
http.HandleFunc("/ws", hub.HandleHTTP())

// UpgradeConnection with options (e.g., set user ID atomically)
client, err := hub.UpgradeConnection(w, r, wshub.WithUserID("user-123"))
```

## Hub Options

| Option | Description |
| --- | --- |
| `WithConfig(cfg)` | Set WebSocket configuration (buffer sizes, timeouts, compression) |
| `WithLogger(l)` | Set a custom logger implementation |
| `WithMetrics(m)` | Set a metrics collector |
| `WithLimits(l)` | Set connection and rate limits |
| `WithHooks(h)` | Set lifecycle hooks |
| `WithMessageHandler(fn)` | Set the message handler function |
| `WithParallelBroadcast(n)` | Enable parallel broadcasting with batch size n |
| `WithParallelBroadcastWorkers(n)` | Set the number of persistent worker goroutines for parallel broadcasting (default: `runtime.NumCPU()`). No effect unless `WithParallelBroadcast` is also set |
| `WithAdapter(adapter)` | Set multi-node adapter for cross-node message delivery |
| `WithNodeID(id)` | Set a stable node identifier for debugging (default: random UUID) |
| `WithPresence(interval)` | Enable periodic presence gossip for global counts |
| `WithHookTimeout(d)` | Max wait for synchronous hooks like `BeforeDisconnect` (default: 5s) |
| `WithDropPolicy(policy)` | Set backpressure behaviour when the send buffer is full (`DropNewest` or `DropOldest`) |
| `WithoutHandlerLatency()` | Disable built-in latency recording (use with `MetricsMiddleware` to avoid double-counting) |
| `WithDrainTimeout(d)` | Max idle time before closing a connection during drain (default: 30s). Set to 0 to disable the idle reaper |

## Broadcasting

| Method | Description |
| --- | --- |
| `Broadcast(data)` | Send text bytes to all connected clients |
| `BroadcastText(text)` | Send a text string to all clients |
| `BroadcastBinary(data)` | Send binary data to all clients |
| `BroadcastJSON(v)` | JSON-encode and send to all clients |
| `BroadcastRawJSON(data)` | Broadcast pre-serialized JSON bytes to all clients (0 allocs, skips marshaling) |
| `BroadcastWithContext(ctx, data)` | Broadcast with context support |
| `BroadcastExcept(data, except...)` | Send text to all except specified clients |
| `BroadcastBinaryExcept(data, except...)` | Send binary to all except specified clients |
| `SendToClient(clientID, data)` | Send text to a specific client by ID |
| `SendBinaryToClient(clientID, data)` | Send binary to a specific client by ID |
| `SendToClientWithContext(ctx, clientID, data)` | Send to client with context (blocks until enqueued) |
| `SendToUser(userID, data)` | Send text to all connections of a user |
| `SendBinaryToUser(userID, data)` | Send binary to all connections of a user |
| `SendToUserWithContext(ctx, userID, data)` | Send to user with context (blocks until enqueued) |

```go
// Broadcast to all connected clients
hub.Broadcast([]byte("hello everyone"))
hub.BroadcastText("hello everyone")
hub.BroadcastJSON(map[string]string{"type": "notification", "msg": "hello"})

// Send to specific client or user
hub.SendToClient(clientID, []byte("private message"))
hub.SendToUser(userID, []byte("sent to all devices"))

// Broadcast to all except certain clients
hub.BroadcastExcept([]byte("hello others"), excludedClient1, excludedClient2)
```

## Client Lookup

| Method | Description |
| --- | --- |
| `Clients()` | Get all connected clients |
| `ClientCount()` | Get count of connected clients (atomic, no lock) |
| `GetClient(id)` | O(1) client lookup by ID |
| `GetClientByUserID(userID)` | Get first client for a user |
| `GetClientsByUserID(userID)` | Get all connections for a user |
| `NodeID()` | Get this hub's unique node identifier |

```go
// Look up clients
client, ok := hub.GetClient(clientID)
if ok {
    client.SendText("found you!")
}

// Multi-device: get all connections for a user
clients := hub.GetClientsByUserID("user-123")
for _, c := range clients {
    c.SendJSON(map[string]string{"type": "sync"})
}

// Count and list
count := hub.ClientCount()
allClients := hub.Clients()
```

## Upgrade Options

Pass per-connection options to `UpgradeConnection` to configure the client
before registration:

```go
// Set user ID atomically during upgrade — before registration.
// This prevents the window where a client exists without a user ID,
// which could bypass MaxConnectionsPerUser limits.
client, err := hub.UpgradeConnection(w, r, wshub.WithUserID(userID))
if err != nil {
    log.Printf("Upgrade failed: %v", err)
    return
}
```

## Drop Policy

Control what happens when a client's send buffer is full:

| Policy | Description |
| --- | --- |
| `DropNewest` | Discard the new message when the buffer is full (default) |
| `DropOldest` | Evict the oldest queued message to make room for the new one |

```go
// Keep the most recent data (good for real-time state updates)
hub := wshub.NewHub(
    wshub.WithDropPolicy(wshub.DropOldest),
    wshub.WithHooks(wshub.Hooks{
        OnSendDropped: func(client *wshub.Client, data []byte) {
            log.Printf("Dropped message for slow client %s", client.ID)
        },
    }),
)
```

## Health and Readiness

Drop-in HTTP handlers for Kubernetes `/healthz` and `/readyz` probes, plus
low-level accessors for building custom health logic.

| Method | Description |
| --- | --- |
| `HealthHandler()` | Returns an `http.HandlerFunc` for `/healthz` — 200 when alive, 503 otherwise |
| `ReadyHandler()` | Returns an `http.HandlerFunc` for `/readyz` — 200 when alive and `StateRunning`, 503 otherwise |
| `Health()` | Returns a `HealthStatus` snapshot (all reads are lock-free atomic loads) |
| `Alive()` | True while the `Run()` goroutine is executing |
| `Ready()` | True when alive and in `StateRunning` (accepting connections) |
| `Uptime()` | Elapsed time since `Run()` started; zero before start or after exit |

Both handlers respond with a JSON body containing `alive`, `ready`, `state`,
`uptime_ns`, and `clients`.

```go
// Register Kubernetes probes
http.HandleFunc("/healthz", hub.HealthHandler()) // liveness
http.HandleFunc("/readyz",  hub.ReadyHandler())  // readiness

// Or use the snapshot for custom logic
h := hub.Health()
fmt.Printf("state=%s uptime=%s clients=%d\n", h.State, h.Uptime, h.Clients)

// Low-level accessors
if !hub.Alive() {
    log.Println("Run() has not started or has exited")
}
if !hub.Ready() {
    log.Println("Hub is draining or stopped — not accepting connections")
}
```

## Graceful Drain

Drain stops accepting new connections (HTTP 503) while letting existing
connections finish in-flight messages. Designed for zero-downtime rolling
deploys (Kubernetes `preStop`, SIGTERM handlers).

| Method | Description |
| --- | --- |
| `Drain(ctx)` | Stop accepting connections and wait for existing ones to disconnect (or the context to expire) |
| `State()` | Returns the current `HubState` (`StateRunning`, `StateDraining`, or `StateStopped`) |
| `IsRunning()` | True when the hub is accepting new connections |
| `IsDraining()` | True when the hub is draining (no new connections, existing ones finishing) |

During drain, idle connections whose send buffers have been empty for the drain
timeout (default 30s, configurable via `WithDrainTimeout`) are proactively
closed with `CloseGoingAway` (1001).

```go
// Kubernetes preStop / SIGTERM handler
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

hub.Drain(ctx)    // stop new connections, wait for existing ones
hub.Shutdown(ctx) // force-close anything remaining
```

### Hub State

| State | Description |
| --- | --- |
| `StateRunning` | Accepting new connections and processing messages normally |
| `StateDraining` | No new connections (HTTP 503), existing connections finishing in-flight messages |
| `StateStopped` | Hub shut down, all connections closed, `Run` loop exited |

```go
// Health/readiness probes
func readinessHandler(w http.ResponseWriter, r *http.Request) {
    if hub.IsRunning() {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

// Check state directly
fmt.Println(hub.State()) // "running", "draining", or "stopped"
```

## Graceful Shutdown

The hub supports context-aware graceful shutdown that closes all client
connections:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Two-phase shutdown for zero-downtime deploys:
// 1. Drain — stop new connections, let existing ones finish
if err := hub.Drain(ctx); err != nil {
    log.Printf("Drain timeout: %v (forcing shutdown)", err)
}
// 2. Shutdown — force-close anything remaining
if err := hub.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```
