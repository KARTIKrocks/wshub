---
id: getting-started
title: Getting Started
description: Install wshub and run a minimal WebSocket server with an echo handler and graceful shutdown.
---

# Getting Started

## Installation

Requires **Go 1.22+**.

```bash
go get github.com/KARTIKrocks/wshub
```

To pin an older release:

```bash
go get github.com/KARTIKrocks/wshub@v1.6.0
```

## Quick Start

A minimal WebSocket server with an echo handler and graceful shutdown:

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
        wshub.WithHooks(wshub.Hooks{
            AfterConnect: func(client *wshub.Client) {
                log.Printf("Client connected: %s", client.ID)
            },
            AfterDisconnect: func(client *wshub.Client) {
                log.Printf("Client disconnected: %s", client.ID)
            },
        }),
    )

    go hub.Run()

    http.HandleFunc("/ws", hub.HandleHTTP())

    srv := &http.Server{Addr: ":8080"}
    go func() {
        log.Println("Listening on :8080")
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    hub.Shutdown(ctx)
    srv.Shutdown(ctx)
}
```

:::note Origin checking

Since v1.7.0 the default origin check is `AllowSameOrigin`. If your front-end is
served from a different origin than the WebSocket endpoint, allowlist it — see
[Configuration → Origin Checking](./configuration.md#origin-checking) — otherwise
the upgrade is rejected with a 403.

:::

## Next steps

- [Hub](./hub.md) — broadcasting, client lookup, drain, and shutdown
- [Rooms](./rooms.md) — grouping clients for targeted broadcasts
- [Router](./router.md) — dispatching messages to per-event handlers
- [Multi-Node Adapters](./adapters.md) — running more than one instance
