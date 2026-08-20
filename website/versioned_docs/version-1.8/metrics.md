---
id: metrics
title: Metrics
description: Pluggable metrics collection for observability, plus an official Prometheus subpackage.
---

# Metrics

Pluggable metrics collection interface for observability. Implement the
`MetricsCollector` interface or use the built-in debug implementation.

```go
import "github.com/KARTIKrocks/wshub"
```

- Pluggable `MetricsCollector` interface
- Built-in `DebugMetrics` implementation for development
- Track connections, messages, latency, errors, and room events
- Official Prometheus subpackage with a drop-in `MetricsCollector`

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#MetricsCollector).

## Metrics Interface

```go
type MetricsCollector interface {
    IncrementConnections()
    DecrementConnections()
    IncrementMessagesReceived()        // 1.5+, was IncrementMessages
    IncrementMessagesSent(count int)   // 1.5+
    IncrementMessagesDropped()         // 1.5+
    RecordMessageSize(size int)
    RecordLatency(duration time.Duration)
    RecordBroadcastDuration(duration time.Duration) // 1.5+
    IncrementErrors(errorType string)
    IncrementRoomJoins()
    IncrementRoomLeaves()
    IncrementRooms()                   // 1.5+
    DecrementRooms()                   // 1.5+
}

// Use with hub
hub := wshub.NewHub(
    wshub.WithMetrics(myCollector),
)
```

## Debug Metrics

Built-in debug implementation for development and testing:

```go
// Create debug metrics collector
metrics := wshub.NewDebugMetrics()

hub := wshub.NewHub(
    wshub.WithMetrics(metrics),
)

// Get a point-in-time stats snapshot
stats := metrics.Stats()
fmt.Printf("Connections: %d\n", stats.Connections)
fmt.Printf("Received: %d\n", stats.TotalMessagesRecv) // renamed from TotalMessages
fmt.Printf("Sent: %d\n", stats.TotalMessagesSent)
fmt.Printf("Dropped: %d\n", stats.TotalDropped)
fmt.Printf("Active rooms: %d\n", stats.ActiveRooms)
fmt.Printf("Avg broadcast: %v\n", stats.AvgBroadcast)

// Pretty-print summary
fmt.Println(metrics)
```

## Prometheus Subpackage

Official drop-in `MetricsCollector` backed by `prometheus/client_golang`.
Import as a separate module:

```bash
go get github.com/KARTIKrocks/wshub/prometheus
```

| Option | Description |
| --- | --- |
| `WithRegistry(reg)` | Use a custom Prometheus registry (default: `prometheus.DefaultRegisterer`) |
| `WithNamespace(ns)` | Set the metric name prefix (default: `wshub`) |
| `WithLatencyBuckets(buckets)` | Custom histogram buckets for `message_latency_seconds` |
| `WithBroadcastBuckets(buckets)` | Custom histogram buckets for `broadcast_duration_seconds` |

Exposed metrics:

| Metric | Description |
| --- | --- |
| `connections_active` | Gauge of currently connected clients |
| `connections_total` | Counter of all connections ever accepted |
| `messages_received_total` | Counter of inbound messages |
| `messages_sent_total` | Counter of outbound messages |
| `messages_dropped_total` | Counter of messages dropped due to full send buffers |
| `message_received_bytes_total` | Counter of inbound bytes |
| `message_latency_seconds` | Histogram of message handler latency |
| `broadcast_duration_seconds` | Histogram of local fanout duration |
| `rooms_active` | Gauge of currently active rooms |
| `room_joins_total` | Counter of room join events |
| `room_leaves_total` | Counter of room leave events |
| `errors_total` | Counter of errors, labelled by `type` |

```go
import whprom "github.com/KARTIKrocks/wshub/prometheus"

// Default setup — registers on prometheus.DefaultRegisterer
collector := whprom.NewCollector()

// Custom registry and namespace
collector := whprom.NewCollector(
    whprom.WithRegistry(myRegistry),
    whprom.WithNamespace("myapp"),
)

hub := wshub.NewHub(
    wshub.WithMetrics(collector),
)
```
