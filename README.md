# wshub


[![Go Reference](https://pkg.go.dev/badge/github.com/KARTIKrocks/wshub.svg)](https://pkg.go.dev/github.com/KARTIKrocks/wshub)
[![CI](https://github.com/KARTIKrocks/wshub/actions/workflows/ci.yml/badge.svg)](https://github.com/KARTIKrocks/wshub/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/KARTIKrocks/wshub)](https://goreportcard.com/report/github.com/KARTIKrocks/wshub)

A production-ready, reusable WebSocket package for Go with support for rooms, broadcasting, middleware, hooks, and extensibility.

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

## JavaScript Client

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  console.log("Connected");

  // Send message
  ws.send(
    JSON.stringify({
      type: "chat",
      message: "Hello!",
    }),
  );
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log("Received:", data);
};

ws.onclose = () => {
  console.log("Disconnected");
};

ws.onerror = (error) => {
  console.error("Error:", error);
};
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

## Thread Safety

All Hub and Client methods are thread-safe. The package uses:

- RWMutex for client/room maps
- Separate mutexes for callbacks
- Channels for cross-goroutine communication
- WaitGroups for graceful shutdown

## License

MIT

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
