---
id: intro
title: Overview
sidebar_label: Overview
description: A production-ready, scalable WebSocket package for Go with rooms, broadcasting, multi-node clustering, middleware, hooks, and extensibility.
slug: /
---

# wshub

A production-ready, scalable WebSocket package for Go with support for rooms,
broadcasting, multi-node clustering, middleware, hooks, and extensibility.

wshub is pure infrastructure. It manages connections, rooms, and message
routing — you bring the business logic.

```bash
go get github.com/KARTIKrocks/wshub
```

## What you get

| | |
| --- | --- |
| **Production-ready** | Proper concurrency, graceful shutdown and drain, error handling |
| **Horizontally scalable** | Multi-node support via adapters (Redis, NATS, or your own) |
| **Pluggable** | Bring your own logger and metrics collector |
| **Middleware** | Chain handlers with cross-cutting logic |
| **Lifecycle hooks** | Hook into connection, message, room, and backpressure events |
| **Rooms** | Group clients for targeted broadcasting |
| **Observability** | Built-in interfaces plus an official Prometheus subpackage |
| **Limits** | Connection, room, and token-bucket rate limits |
| **Backpressure** | Configurable drop policies with notification hooks |
| **Health probes** | `/healthz` and `/readyz` handlers for Kubernetes |
| **Thread safe** | Every method is safe for concurrent use |

## Where to go next

- **[Getting Started](./getting-started.md)** — install and run a minimal server
- **[Hub](./hub.md)** — the central connection manager
- **[Multi-Node Adapters](./adapters.md)** — scaling past one process
- **[API Reference](https://pkg.go.dev/github.com/KARTIKrocks/wshub)** — full
  generated godoc on pkg.go.dev

## Documentation layout

These guides explain concepts, patterns, and configuration. For exact type
signatures, method sets, and struct fields, use
[pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub) — it is generated
from the source and is always authoritative.

## Requirements

Go 1.27 or later. _Changed in 1.8: previously Go 1.22 or later._
