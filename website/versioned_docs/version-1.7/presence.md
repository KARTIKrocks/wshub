---
id: presence
title: Presence
description: Periodic presence gossip for cluster-wide client and room counts in multi-node deployments.
---

# Presence

Periodic presence gossip for cluster-wide client and room counts in multi-node
deployments.

```go
import "github.com/KARTIKrocks/wshub"
```

- Periodic heartbeat publishing of local client/room counts
- Cluster-wide totals via `GlobalClientCount` and `GlobalRoomCount`
- Automatic eviction of stale nodes (3 missed heartbeats)
- O(1) change detection avoids re-gathering unchanged stats
- Requires an [adapter](./adapters.md) — no-op in single-node mode

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub).

## Enabling Presence

Enable presence with `WithPresence` alongside an adapter. Each hub publishes its
local stats at the given interval:

```go
hub := wshub.NewHub(
    wshub.WithAdapter(adapter),        // required for presence
    wshub.WithPresence(5*time.Second), // heartbeat interval (default: 5s)
    wshub.WithNodeID("node-1"),        // stable ID for debugging
)
go hub.Run()
```

When the interval is zero, the default of 5 seconds is used. Nodes that miss 3
consecutive heartbeats are automatically evicted from the totals.

## Global Counts

| Method | Description |
| --- | --- |
| `GlobalClientCount()` | Total connected clients across all nodes |
| `GlobalRoomCount(room)` | Total clients in a room across all nodes |

```go
// Get cluster-wide totals
totalClients := hub.GlobalClientCount()
totalInRoom := hub.GlobalRoomCount("chat-general")

// In single-node mode (no adapter/presence), these return local counts
localClients := hub.ClientCount()        // always local
globalClients := hub.GlobalClientCount() // local + remote when presence enabled
```

## Full Example

A multi-node setup with the Redis adapter and presence:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/KARTIKrocks/wshub"
    wshubredis "github.com/KARTIKrocks/wshub/adapter/redis"
    goredis "github.com/redis/go-redis/v9"
)

func main() {
    rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
    adapter := wshubredis.New(rdb)

    nodeID, _ := os.Hostname()
    hub := wshub.NewHub(
        wshub.WithAdapter(adapter),
        wshub.WithPresence(5*time.Second),
        wshub.WithNodeID(nodeID),
        wshub.WithMessageHandler(func(c *wshub.Client, msg *wshub.Message) error {
            hub.Broadcast(msg.Data)
            return nil
        }),
    )
    go hub.Run()

    // Expose cluster stats
    http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "local: %d, global: %d\n",
            hub.ClientCount(), hub.GlobalClientCount())
    })
    http.HandleFunc("/ws", hub.HandleHTTP())

    srv := &http.Server{Addr: ":8080"}
    go srv.ListenAndServe()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    hub.Shutdown(ctx)
    srv.Shutdown(ctx)
}
```
