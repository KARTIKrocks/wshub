---
id: limits
title: Limits
description: Control connections, rooms, and message rates to protect your server from abuse.
---

# Limits

Control connections, rooms, and message rates to protect your server from abuse.

```go
import "github.com/KARTIKrocks/wshub"
```

- Maximum total connections
- Per-user connection limits (multi-device control)
- Per-client room limits
- Per-room client limits
- Per-client message rate limiting

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Limits).

## Connection Limits

```go
limits := wshub.DefaultLimits().
    WithMaxConnections(10000).       // max total connections
    WithMaxConnectionsPerUser(5)     // max connections per user ID

hub := wshub.NewHub(
    wshub.WithLimits(limits),
)
```

## Room Limits

```go
limits := wshub.DefaultLimits().
    WithMaxRoomsPerClient(10).   // max rooms a client can join
    WithMaxClientsPerRoom(100)   // max clients per room
```

## Rate Limiting

Per-client message rate limiting using a token-bucket algorithm. Tokens refill
at `MaxMessageRate` per second, capped at a burst of `MaxMessageRate`. This
provides smoother throttling than fixed windows:

```go
limits := wshub.DefaultLimits().
    WithMaxMessageRate(100) // 100 tokens/sec, burst of 100

// Complete limits example
limits := wshub.DefaultLimits().
    WithMaxConnections(10000).
    WithMaxConnectionsPerUser(5).
    WithMaxRoomsPerClient(10).
    WithMaxClientsPerRoom(100).
    WithMaxMessageRate(100)

hub := wshub.NewHub(
    wshub.WithLimits(limits),
)
```

Exceeding a limit returns one of the [limit errors](./errors.md#limit-errors).
