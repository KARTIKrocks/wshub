# WebSocket Common Package - Implementation Guide

## 📦 Package Structure

```
websocket-pkg/
├── config.go          # Configuration and message types
├── errors.go          # All error definitions
├── logger.go          # Logger interface
├── metrics.go         # Metrics interface
├── codec.go           # Encoding/decoding interface
├── hooks.go           # Lifecycle hooks
├── limits.go          # Connection and rate limits
├── middleware.go      # Middleware system
├── message.go         # Message types
├── client.go          # Client connection management
├── hub.go             # Hub for managing clients
├── go.mod             # Go module file
├── README.md          # Comprehensive documentation
├── USAGE.md           # Integration examples
└── examples/
    ├── simple/        # Basic echo server
    │   └── main.go
    └── chat/          # Advanced chat application
        └── main.go
```

## ✅ What's Included (Core Infrastructure)

### 1. **Connection Management**

- Thread-safe client registry
- Graceful connection handling
- Automatic cleanup on disconnect
- Support for multiple connections per user

### 2. **Room System**

- Join/leave rooms
- Broadcast to specific rooms
- Room-based filtering
- Automatic cleanup of empty rooms

### 3. **Middleware System**

- Chainable middleware
- Built-in logging, recovery, and metrics middleware
- Easy to create custom middleware

### 4. **Hooks System**

- BeforeConnect - Authentication, authorization
- AfterConnect - User setup, notifications
- BeforeMessage/AfterMessage - Validation, logging
- BeforeRoomJoin/AfterRoomJoin - Permission checks
- OnError - Error handling

### 5. **Pluggable Interfaces**

- Logger - Plug in Zap, Logrus, Slog, etc.
- Metrics - Plug in Prometheus, StatsD, etc.
- Codec - JSON by default, extensible to MessagePack, Protobuf

### 6. **Configuration**

- Builder pattern for easy configuration
- Sensible defaults
- Origin checking (CORS)
- Message size limits
- Timeouts and ping/pong

### 7. **Limits & Safety**

- Max connections (global and per-user)
- Max rooms per client
- Max clients per room
- Message rate limiting support

### 8. **Concurrency**

- All operations are thread-safe
- Proper mutex usage
- WaitGroups for graceful shutdown
- Context-based cancellation

## ❌ What's NOT Included (Business Logic)

The package intentionally **excludes**:

- ❌ Message schemas/formats (define your own)
- ❌ Authentication logic (implement via hooks)
- ❌ Authorization logic (implement via middleware)
- ❌ Presence tracking (implement in your app)
- ❌ Message persistence (integrate your DB)
- ❌ Event routing (define your own events)
- ❌ Specific response formats (define your own)
- ❌ Business rules (implement in your handlers)

## 🚀 How to Use in Your Projects

### Step 1: Copy to Your Project

```bash
# Copy the entire package to your project
cp -r websocket-pkg/ /path/to/yourproject/pkg/websocket/

# Or create as a separate Go module
cd websocket-pkg
go mod init github.com/yourcompany/websocket
```

### Step 2: Update Import Path

If using as internal package:

```go
import "yourproject/pkg/websocket"
```

If using as separate module:

```go
import "github.com/yourcompany/websocket"
```

### Step 3: Create Your Application Layer

Create a wrapper in your project:

```go
// internal/chat/server.go
package chat

import (
    "yourproject/pkg/websocket"
    "go.uber.org/zap"
)

type ChatServer struct {
    hub         *websocket.Hub
    logger      *zap.Logger
    db          *sql.DB
    authService *AuthService
}

func NewChatServer(logger *zap.Logger, db *sql.DB, auth *AuthService) *ChatServer {
    config := websocket.DefaultConfig().
        WithMaxMessageSize(1024 * 1024).
        WithCompression(true)

    hub := websocket.NewHub(config)

    s := &ChatServer{
        hub:         hub,
        logger:      logger,
        db:          db,
        authService: auth,
    }

    // Setup your business logic
    s.setupHooks()
    s.setupHandlers()

    return s
}

func (s *ChatServer) setupHooks() {
    s.hub.SetHooks(websocket.Hooks{
        BeforeConnect: s.authenticate,
        AfterConnect:  s.onUserConnect,
        // ... your hooks
    })
}

func (s *ChatServer) setupHandlers() {
    chain := websocket.NewMiddlewareChain(s.handleMessage).
        Use(websocket.RecoveryMiddleware(&ZapLogger{s.logger})).
        Use(RateLimitMiddleware(s.rateLimiter)).
        Use(AuthMiddleware(s.authService))

    s.hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
        return chain.Execute(client, msg)
    })
}

// Your business logic
func (s *ChatServer) handleMessage(client *websocket.Client, msg *websocket.Message) error {
    // Parse your message format
    // Apply your business rules
    // Save to database
    // Broadcast to rooms
    // etc.
}
```

## 📋 Migration Checklist

If you're migrating from the old code:

- [x] Remove `server.go` - create your own wrapper
- [x] Remove `Presence` - implement as needed
- [x] Remove `JSONResponse` helpers - define your own
- [x] Remove `Router/EventHandler` - define your own
- [x] Update to use hooks instead of direct callbacks
- [x] Implement logger interface for your logger
- [x] Implement metrics interface for your metrics
- [x] Update middleware to use new system
- [x] Use context-based shutdown

## 🔧 Key Improvements Over Original

### 1. **Better Concurrency**

- Protected callback setters
- WaitGroups for shutdown
- Context support throughout

### 2. **Extensibility**

- Logger interface (was hardcoded)
- Metrics interface (was hardcoded)
- Codec interface (was JSON only)
- Hooks system (more flexible than callbacks)

### 3. **Better Separation**

- No business logic in core package
- Middleware system for cross-cutting concerns
- Clean interfaces for plugging in dependencies

### 4. **Production Features**

- Graceful shutdown with timeout
- Connection limits
- Rate limiting support
- Better error handling
- Logging throughout

### 5. **Developer Experience**

- Comprehensive documentation
- Multiple examples
- Clear usage patterns
- Integration guides

## 🎯 Common Use Cases

### Use Case 1: Chat Application

```go
// See examples/chat/main.go for complete example
- User authentication via hooks
- Room management
- Message persistence
- Presence tracking (custom)
```

### Use Case 2: Real-time Dashboard

```go
- Metrics streaming
- Alert notifications
- Live data updates
- Multiple dashboard instances per user
```

### Use Case 3: Collaborative Editing

```go
- Document rooms
- Operational transformations
- Cursor positions
- Conflict resolution
```

### Use Case 4: IoT Device Communication

```go
- Device authentication
- Command dispatch
- Status updates
- Device groups (rooms)
```

### Use Case 5: Gaming

```go
- Game lobbies (rooms)
- Player state sync
- Real-time events
- Matchmaking
```

## 📊 Performance Considerations

### Scalability Tips

1. **Increase buffer sizes** for high throughput:

   ```go
   config.WithSendChannelSize(1024)
   ```

2. **Enable compression** for large messages:

   ```go
   config.WithCompression(true)
   ```

3. **Set appropriate limits**:

   ```go
   limits.WithMaxConnections(10000).
          WithMaxConnectionsPerUser(5)
   ```

4. **Use context timeouts**:

   ```go
   ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
   hub.BroadcastWithContext(ctx, data)
   ```

5. **Monitor metrics**:
   - Connection count
   - Message rate
   - Buffer fullness
   - Error rates

## 🧪 Testing

### Unit Tests

```go
// Test your business logic
func TestChatMessageHandler(t *testing.T) {
    server := NewChatServer(...)
    client := &websocket.Client{ID: "test"}
    msg := &websocket.Message{Data: []byte(`{"type":"chat"}`)}

    err := server.handleMessage(client, msg)
    assert.NoError(t, err)
}
```

### Integration Tests

```go
// Test WebSocket connections
func TestWebSocketConnection(t *testing.T) {
    hub := websocket.NewHub(websocket.DefaultConfig())
    go hub.Run()

    server := httptest.NewServer(http.HandlerFunc(hub.HandleHTTP()))

    // Connect client and test
}
```

## 🔒 Security Best Practices

1. **Always validate origin** in production:

   ```go
   config.WithCheckOrigin(websocket.AllowOrigins("https://yourdomain.com"))
   ```

2. **Implement authentication** in BeforeConnect hook
3. **Rate limit** using middleware
4. **Validate message size** using config
5. **Sanitize user input** in message handlers
6. **Use TLS** in production (wss://)

## 📝 Next Steps

1. Copy the package to your project
2. Update import paths
3. Create your application wrapper
4. Implement logger adapter
5. Implement metrics adapter
6. Define your message formats
7. Implement business logic
8. Test thoroughly
9. Deploy!

## 🆘 Support

For questions or issues:

1. Check README.md for documentation
2. Review USAGE.md for integration examples
3. Look at examples/ for working code
4. Review the inline code comments

## 📄 License

MIT License - Use freely in your projects!
