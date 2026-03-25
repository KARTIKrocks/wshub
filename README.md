# wshub

[![Go Reference](https://pkg.go.dev/badge/github.com/KARTIKrocks/wshub.svg)](https://pkg.go.dev/github.com/KARTIKrocks/wshub)
[![Go Report Card](https://goreportcard.com/badge/github.com/KARTIKrocks/wshub)](https://goreportcard.com/report/github.com/KARTIKrocks/wshub)
[![Go Version](https://img.shields.io/github/go-mod/go-version/KARTIKrocks/wshub)](go.mod)
[![CI](https://github.com/KARTIKrocks/wshub/actions/workflows/ci.yml/badge.svg)](https://github.com/KARTIKrocks/wshub/actions/workflows/ci.yml)
[![GitHub tag](https://img.shields.io/github/v/tag/KARTIKrocks/wshub)](https://github.com/KARTIKrocks/wshub/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![codecov](https://codecov.io/gh/KARTIKrocks/wshub/branch/main/graph/badge.svg)](https://codecov.io/gh/KARTIKrocks/wshub)

A production-ready, reusable WebSocket package for Go with support for rooms, broadcasting, middleware, hooks, and extensibility.

**[Documentation](https://kartikrocks.github.io/wshub/)** | **[API Reference](https://pkg.go.dev/github.com/KARTIKrocks/wshub)**

## Features

- **Production-Ready**: Proper concurrency, graceful shutdown, error handling
- **Pluggable**: Bring your own logger, metrics
- **Middleware System**: Chain handlers with custom logic
- **Lifecycle Hooks**: Hook into connection, message, and room events
- **Room Support**: Group clients into rooms for targeted broadcasting
- **Metrics & Logging**: Built-in interfaces for observability
- **Configurable**: Extensive configuration with builder pattern
- **Limits & Rate Limiting**: Control connections, rooms, and message rates
- **Zero Business Logic**: Pure infrastructure, bring your own logic

## Performance Highlights

Zero-allocation broadcasting, nanosecond lookups — built for scale. ([Full benchmarks](#benchmarks))

| Operation                | Scale             | Time    | Allocs |
| ------------------------ | ----------------- | ------- | ------ |
| `Broadcast`              | 100,000 clients   | 49.5 ms | 0      |
| `Broadcast`              | 1,000,000 clients | 1.12 s  | 0      |
| `GetClient`              | 1,000 clients     | 23.5 ns | 0      |
| `GetClientByUserID`      | 100 users         | 66.6 ns | 0      |
| Middleware chain (built) | 3 middlewares     | 7.1 ns  | 0      |

> Message size has no impact on dispatch — 64 B and 64 KB both take 6.2 μs for 100 clients.

## Installation

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
    "time"

    "github.com/KARTIKrocks/wshub"
)

func main() {
    // Create hub with configuration
    config := wshub.DefaultConfig().
        WithMaxMessageSize(1024 * 1024).
        WithCompression(true)

    hub := wshub.NewHub(
        wshub.WithConfig(config),
        wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
            log.Printf("Message from %s: %s", client.ID, msg.Text())
            return client.Send(msg.Data)
        }),
    )

    // Start the hub
    go hub.Run()

    // Set up HTTP handler
    http.HandleFunc("/ws", hub.HandleHTTP())

    log.Println("Server starting on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal(err)
    }

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    hub.Shutdown(ctx)
}
```

## Configuration

### Basic Configuration

```go
config := wshub.DefaultConfig()

// Or customize
config := wshub.Config{
    ReadBufferSize:    4096,
    WriteBufferSize:   4096,
    WriteWait:         10 * time.Second,
    PongWait:          60 * time.Second,
    PingPeriod:        54 * time.Second,
    MaxMessageSize:    1024 * 1024,
    SendChannelSize:   512,
    EnableCompression: true,
    CheckOrigin:       wshub.AllowAllOrigins,
}
```

### Builder Pattern

```go
config := wshub.DefaultConfig().
    WithBufferSizes(4096, 4096).
    WithMaxMessageSize(1024 * 1024).
    WithCompression(true).
    WithCheckOrigin(wshub.AllowOrigins("https://example.com"))
```

### Origin Checking

```go
// Allow all origins (default)
config.CheckOrigin = wshub.AllowAllOrigins

// Allow same origin only
config.CheckOrigin = wshub.AllowSameOrigin

// Allow specific origins
config.CheckOrigin = wshub.AllowOrigins("https://example.com", "https://app.example.com")

// Custom checker
config.CheckOrigin = func(r *http.Request) bool {
    return strings.HasSuffix(r.Header.Get("Origin"), ".example.com")
}
```

## Hub API

### Client Management

```go
// Get all clients
clients := hub.Clients()
count := hub.ClientCount()

// Find client
client, ok := hub.GetClient(clientID)
client, ok := hub.GetClientByUserID(userID)
clients := hub.GetClientsByUserID(userID)
```

### Broadcasting

```go
// Broadcast to all
hub.Broadcast([]byte("Hello everyone"))
hub.BroadcastText("Hello everyone")
hub.BroadcastJSON(map[string]string{"message": "Hello"})

// Broadcast with context
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
hub.BroadcastWithContext(ctx, data)

// Broadcast except one client
hub.BroadcastExcept(data, excludeClient)

// Send to specific client
hub.SendToClient(clientID, data)

// Send to all connections of a user
hub.SendToUser(userID, data)
```

### Rooms

```go
// Join/leave rooms
hub.JoinRoom(client, "general")
hub.LeaveRoom(client, "general")
hub.LeaveAllRooms(client)

// Broadcast to room
hub.BroadcastToRoom("general", data)
hub.BroadcastToRoomExcept("general", data, exceptClient)

// Room info
clients := hub.RoomClients("general")
count := hub.RoomCount("general")
rooms := hub.RoomNames()
exists := hub.RoomExists("general")
```

## Client API

```go
// Client properties
client.ID       // Unique client ID

// Set user ID
client.SetUserID("user-123")
userID := client.GetUserID()

// Metadata
client.SetMetadata("role", "admin")
role, ok := client.GetMetadata("role")
client.DeleteMetadata("role")

// Send messages
client.Send([]byte("Hello"))
client.SendText("Hello")
client.SendJSON(map[string]string{"message": "Hello"})
client.SendBinary(data)
client.SendWithContext(ctx, data)

// Close connection
client.Close()
client.CloseWithCode(websocket.CloseNormalClosure, "Goodbye")

// Room membership
rooms := client.Rooms()
inRoom := client.InRoom("general")
count := client.RoomCount()

// Status
closed := client.IsClosed()
closedAt := client.ClosedAt()

// Client-specific handlers
client.OnMessage(func(c *wshub.Client, msg *wshub.Message) {
    // Handle message
})

client.OnClose(func(c *wshub.Client) {
    // Handle close
})

client.OnError(func(c *wshub.Client, err error) {
    // Handle error
})
```

## Hooks System

```go
hub := wshub.NewHub(
    wshub.WithHooks(wshub.Hooks{
        // Before connection upgrade
        BeforeConnect: func(r *http.Request) error {
            token := r.Header.Get("Authorization")
            if !validateToken(token) {
                return wshub.ErrAuthenticationFailed
            }
            return nil
        },

        // After successful connection
        AfterConnect: func(client *wshub.Client) {
            log.Printf("Client connected: %s", client.ID)
        },

        // Before message processing
        BeforeMessage: func(client *wshub.Client, msg *wshub.Message) (*wshub.Message, error) {
            if len(msg.Data) > 1000 {
                return nil, errors.New("message too large")
            }
            return msg, nil
        },

        // After message processing
        AfterMessage: func(client *wshub.Client, msg *wshub.Message, err error) {
            if err != nil {
                log.Printf("Message error: %v", err)
            }
        },

        // Before room join
        BeforeRoomJoin: func(client *wshub.Client, room string) error {
            if !canJoinRoom(client, room) {
                return wshub.ErrUnauthorized
            }
            return nil
        },

        // After room join
        AfterRoomJoin: func(client *wshub.Client, room string) {
            hub.BroadcastToRoomExcept(room,
                []byte(fmt.Sprintf("%s joined", client.ID)),
                client,
            )
        },

        // On error
        OnError: func(client *wshub.Client, err error) {
            log.Printf("Client error: %v", err)
        },
    }),
)
```

## Middleware System

```go
// Create middleware chain
chain := wshub.NewMiddlewareChain(handleMessage).
    Use(wshub.RecoveryMiddleware(logger)).
    Use(wshub.LoggingMiddleware(logger)).
    Use(wshub.MetricsMiddleware(metrics)).
    Build()

// Use in message handler
hub := wshub.NewHub(
    wshub.WithMessageHandler(chain.Execute),
)
```

### Built-in Middlewares

```go
// Logging
wshub.LoggingMiddleware(logger)

// Panic recovery
wshub.RecoveryMiddleware(logger)

// Metrics
wshub.MetricsMiddleware(metrics)
```

### Custom Middleware

```go
func RateLimitMiddleware(limiter RateLimiter) wshub.Middleware {
    return func(next wshub.HandlerFunc) wshub.HandlerFunc {
        return func(client *wshub.Client, msg *wshub.Message) error {
            if !limiter.Allow(client.ID) {
                return wshub.ErrRateLimitExceeded
            }
            return next(client, msg)
        }
    }
}

func AuthMiddleware(auth AuthService) wshub.Middleware {
    return func(next wshub.HandlerFunc) wshub.HandlerFunc {
        return func(client *wshub.Client, msg *wshub.Message) error {
            if client.GetUserID() == "" {
                return wshub.ErrUnauthorized
            }
            return next(client, msg)
        }
    }
}
```

## Logging

```go
// Implement the Logger interface
type ZapLogger struct {
    logger *zap.Logger
}

func (l *ZapLogger) Debug(msg string, args ...any) {
    l.logger.Sugar().Debugw(msg, args...)
}

func (l *ZapLogger) Info(msg string, args ...any) {
    l.logger.Sugar().Infow(msg, args...)
}

func (l *ZapLogger) Warn(msg string, args ...any) {
    l.logger.Sugar().Warnw(msg, args...)
}

func (l *ZapLogger) Error(msg string, args ...any) {
    l.logger.Sugar().Errorw(msg, args...)
}

// Use it
hub := wshub.NewHub(wshub.WithLogger(&ZapLogger{logger}))
```

## Metrics

```go
// Implement the MetricsCollector interface
type PrometheusMetrics struct {
    connections   prometheus.Gauge
    messages      prometheus.Counter
    messageSize   prometheus.Histogram
    errors        *prometheus.CounterVec
}

func (m *PrometheusMetrics) IncrementConnections() {
    m.connections.Inc()
}

func (m *PrometheusMetrics) IncrementMessages() {
    m.messages.Inc()
}

func (m *PrometheusMetrics) RecordMessageSize(size int) {
    m.messageSize.Observe(float64(size))
}

func (m *PrometheusMetrics) IncrementErrors(errorType string) {
    m.errors.WithLabelValues(errorType).Inc()
}

// ... implement other methods

// Use it
hub := wshub.NewHub(wshub.WithMetrics(NewPrometheusMetrics()))
```

## Limits

```go
limits := wshub.DefaultLimits().
    WithMaxConnections(10000).
    WithMaxConnectionsPerUser(5).
    WithMaxRoomsPerClient(10).
    WithMaxClientsPerRoom(100).
    WithMaxMessageRate(100)

hub := wshub.NewHub(wshub.WithLimits(limits))
```

## Error Handling

```go
err := hub.JoinRoom(client, room)
switch err {
case wshub.ErrClientNotFound:
    // Client not registered
case wshub.ErrAlreadyInRoom:
    // Client already in room
case wshub.ErrEmptyRoomName:
    // Empty room name
case wshub.ErrRoomNotFound:
    // Room doesn't exist
case wshub.ErrNotInRoom:
    // Client not in room
case wshub.ErrConnectionClosed:
    // Connection was closed
case wshub.ErrWriteTimeout:
    // Write buffer full
case wshub.ErrMaxConnectionsReached:
    // Connection limit reached
case wshub.ErrMaxRoomsReached:
    // Room limit per client reached
case wshub.ErrRoomFull:
    // Room is full
case wshub.ErrRateLimitExceeded:
    // Rate limit exceeded
case wshub.ErrAuthenticationFailed:
    // Authentication failed
case wshub.ErrUnauthorized:
    // Unauthorized action
}
```

## Complete Example: Chat Application

See [examples/chat/](examples/chat/) for a complete chat application demonstrating:

- Room management
- Username tracking
- Message broadcasting
- Middleware (recovery + logging)
- Rate limiting
- Connection limits

## Test Client

Save as `index.html` and open in a browser while the server is running:

```html
<!DOCTYPE html>
<html>
  <head>
    <title>WebSocket Test</title>
  </head>
  <body>
    <h1>WebSocket Test</h1>
    <div>
      <input type="text" id="message" placeholder="Type a message" />
      <button onclick="send()">Send</button>
    </div>
    <div id="messages"></div>

    <script>
      const ws = new WebSocket("ws://localhost:8080/ws");

      ws.onopen = () => {
        console.log("Connected");
        addMessage("Connected to server");
      };

      ws.onmessage = (event) => {
        addMessage("Received: " + event.data);
      };

      ws.onclose = () => {
        addMessage("Disconnected");
      };

      ws.onerror = (error) => {
        console.error("WebSocket error:", error);
        addMessage("Error occurred");
      };

      function send() {
        const input = document.getElementById("message");
        ws.send(input.value);
        addMessage("Sent: " + input.value);
        input.value = "";
      }

      function addMessage(msg) {
        const div = document.getElementById("messages");
        div.innerHTML += "<p>" + msg + "</p>";
      }
    </script>
  </body>
</html>
```

## Best Practices

1. **Always use middleware for cross-cutting concerns** (logging, metrics, auth)
2. **Use hooks for lifecycle events** instead of wrapping the hub
3. **Implement proper logging and metrics** for production observability
4. **Set appropriate limits** to prevent resource exhaustion
5. **Use graceful shutdown** with context timeout
6. **Handle errors appropriately** - don't ignore send failures
7. **Use rooms for targeted messaging** instead of filtering in handlers
8. **Set user ID after authentication** for multi-device support
9. **Use metadata for request-scoped data** instead of global state
10. **Test with concurrent clients** to ensure thread safety

## Performance Tips

- Increase `SendChannelSize` for high-throughput scenarios
- Enable compression for large messages
- Use `BroadcastWithContext` for timeout control
- Batch messages when possible
- Monitor send buffer sizes via metrics
- Use `WithParallelBroadcast` for 1000+ concurrent clients

## Benchmarks

Measured on an Intel i5-11400H @ 2.70GHz (12 cores), Go 1.24, Linux. See [performance highlights](#performance-highlights) for a quick summary.

Run them yourself:

```bash
go test -bench=. -benchmem ./...
```

### Broadcasting (zero allocations)

| Operation               | Clients   | Time    | Allocs |
| ----------------------- | --------- | ------- | ------ |
| `Broadcast`             | 5,000     | 778 μs  | 0      |
| `Broadcast`             | 100,000   | 49.5 ms | 0      |
| `Broadcast`             | 1,000,000 | 1.12 s  | 0      |
| `BroadcastToRoom`       | 5,000     | 754 μs  | 0      |
| `BroadcastToRoom`       | 100,000   | 50.3 ms | 0      |
| `BroadcastToRoom`       | 1,000,000 | 670 ms  | 0      |
| `BroadcastExcept`       | 5,000     | 774 μs  | 0      |
| `BroadcastExcept`       | 100,000   | 53.2 ms | 0      |
| `BroadcastExcept`       | 1,000,000 | 647 ms  | 0      |
| `BroadcastToRoomExcept` | 5,000     | 744 μs  | 0      |
| `BroadcastToRoomExcept` | 100,000   | 51.1 ms | 0      |
| `BroadcastToRoomExcept` | 1,000,000 | 826 ms  | 0      |

### Message size has no impact on dispatch

| Payload | Time (100 clients) | Allocs |
| ------- | ------------------ | ------ |
| 64 B    | 6.2 μs             | 0      |
| 512 B   | 6.2 μs             | 0      |
| 4 KB    | 6.2 μs             | 0      |
| 64 KB   | 6.2 μs             | 0      |

### Client & Room Lookups (zero allocations)

| Operation                   | Time    | Allocs |
| --------------------------- | ------- | ------ |
| `GetClient` (1,000 clients) | 23.5 ns | 0      |
| `ClientCount`               | 17.8 ns | 0      |
| `GetClientByUserID`         | 66.6 ns | 0      |
| `RoomExists`                | 22.5 ns | 0      |
| `RoomCount`                 | 37.7 ns | 0      |
| `GetMetadata`               | 22.8 ns | 0      |
| `SetMetadata`               | 40.0 ns | 0      |

### Client Send

| Operation     | Time   | Allocs |
| ------------- | ------ | ------ |
| `Send` (text) | 101 ns | 1      |
| `SendJSON`    | 569 ns | 5      |

### Middleware Chain

| Mode                 | Time   | Allocs |
| -------------------- | ------ | ------ |
| Built (cached)       | 7.1 ns | 0      |
| Unbuilt (on-the-fly) | 171 ns | 3      |

> Always call `Build()` on your middleware chain — it's **24x faster**.

### Concurrent Access (parallel goroutines)

| Operation                 | Time    | Allocs |
| ------------------------- | ------- | ------ |
| `GetClient`               | 32.9 ns | 0      |
| `ClientCount`             | 31.8 ns | 0      |
| `Metadata` (set+get)      | 93.2 ns | 0      |
| `Broadcast` (100 clients) | 7.3 μs  | 111    |

### Message Creation

| Operation          | Time    | Allocs |
| ------------------ | ------- | ------ |
| `NewMessage`       | 173 ns  | 1      |
| `NewTextMessage`   | 222 ns  | 2      |
| `NewBinaryMessage` | 174 ns  | 1      |
| `NewJSONMessage`   | 1.78 μs | 9      |

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
