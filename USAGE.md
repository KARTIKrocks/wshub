# Usage Guide

## Integration in Your Projects

### 1. Basic Integration

```go
// In your project: internal/websocket/server.go
package websocket

import (
    "github.com/yourusername/websocket"
    "go.uber.org/zap"
)

type Server struct {
    hub    *websocket.Hub
    logger *zap.Logger
    // Your business logic dependencies
    userService *UserService
    authService *AuthService
}

func NewServer(logger *zap.Logger, userService *UserService, authService *AuthService) *Server {
    config := websocket.DefaultConfig().
        WithMaxMessageSize(1024 * 1024).
        WithCompression(true)
    
    hub := websocket.NewHub(config)
    
    s := &Server{
        hub:         hub,
        logger:      logger,
        userService: userService,
        authService: authService,
    }
    
    s.setupHub()
    return s
}

func (s *Server) setupHub() {
    // Set logger
    hub.SetLogger(&ZapLoggerAdapter{s.logger})
    
    // Set metrics
    hub.SetMetrics(NewPrometheusMetrics())
    
    // Set hooks
    s.setupHooks()
    
    // Set message handler
    s.setupMessageHandler()
}

func (s *Server) setupHooks() {
    s.hub.SetHooks(websocket.Hooks{
        BeforeConnect: s.authenticateConnection,
        AfterConnect:  s.onConnect,
        AfterDisconnect: s.onDisconnect,
    })
}

func (s *Server) authenticateConnection(r *http.Request) error {
    token := r.Header.Get("Authorization")
    user, err := s.authService.ValidateToken(token)
    if err != nil {
        return websocket.ErrAuthenticationFailed
    }
    
    // Store user in request context for later use
    // You'll access this in AfterConnect
    return nil
}

func (s *Server) onConnect(client *websocket.Client) {
    // Set user ID from your auth system
    userID := // ... extract from your auth
    client.SetUserID(userID)
    
    // Load user data and store in metadata
    user, _ := s.userService.GetUser(userID)
    client.SetMetadata("user", user)
    
    s.logger.Info("User connected",
        zap.String("clientID", client.ID),
        zap.String("userID", userID),
    )
}
```

### 2. With Database Integration

```go
package websocket

import (
    "database/sql"
    "encoding/json"
)

type ChatServer struct {
    hub *websocket.Hub
    db  *sql.DB
}

func (s *ChatServer) handleChatMessage(client *websocket.Client, msg *websocket.Message) error {
    var chatMsg ChatMessage
    if err := json.Unmarshal(msg.Data, &chatMsg); err != nil {
        return err
    }
    
    // Save message to database
    messageID, err := s.saveMessage(client.GetUserID(), chatMsg)
    if err != nil {
        return err
    }
    
    // Add message ID to response
    chatMsg.ID = messageID
    
    // Broadcast to room
    data, _ := json.Marshal(chatMsg)
    return s.hub.BroadcastToRoom(chatMsg.RoomID, data)
}

func (s *ChatServer) saveMessage(userID string, msg ChatMessage) (int64, error) {
    result, err := s.db.Exec(
        "INSERT INTO messages (user_id, room_id, content, created_at) VALUES (?, ?, ?, NOW())",
        userID, msg.RoomID, msg.Content,
    )
    if err != nil {
        return 0, err
    }
    return result.LastInsertId()
}
```

### 3. With Redis for Multi-Server Setup

```go
package websocket

import (
    "context"
    "encoding/json"
    
    "github.com/redis/go-redis/v9"
    "github.com/yourusername/websocket"
)

type DistributedServer struct {
    hub   *websocket.Hub
    redis *redis.Client
}

func NewDistributedServer(redisClient *redis.Client) *DistributedServer {
    hub := websocket.NewHub(websocket.DefaultConfig())
    
    s := &DistributedServer{
        hub:   hub,
        redis: redisClient,
    }
    
    // Subscribe to Redis pub/sub
    go s.subscribeToRedis()
    
    // When receiving local messages, publish to Redis
    hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
        return s.publishToRedis(msg)
    })
    
    return s
}

func (s *DistributedServer) subscribeToRedis() {
    pubsub := s.redis.Subscribe(context.Background(), "websocket:broadcast")
    
    for msg := range pubsub.Channel() {
        var wsMsg websocket.Message
        if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
            continue
        }
        
        // Broadcast to local connections
        s.hub.Broadcast(wsMsg.Data)
    }
}

func (s *DistributedServer) publishToRedis(msg *websocket.Message) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    return s.redis.Publish(context.Background(), "websocket:broadcast", data).Err()
}
```

### 4. With gRPC Backend

```go
package websocket

import (
    "context"
    
    pb "your/proto/package"
    "github.com/yourusername/websocket"
    "google.golang.org/grpc"
)

type GRPCWebSocketBridge struct {
    hub        *websocket.Hub
    grpcClient pb.ChatServiceClient
}

func NewGRPCBridge(grpcConn *grpc.ClientConn) *GRPCWebSocketBridge {
    hub := websocket.NewHub(websocket.DefaultConfig())
    
    bridge := &GRPCWebSocketBridge{
        hub:        hub,
        grpcClient: pb.NewChatServiceClient(grpcConn),
    }
    
    hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
        return bridge.forwardToGRPC(client, msg)
    })
    
    return bridge
}

func (b *GRPCWebSocketBridge) forwardToGRPC(client *websocket.Client, msg *websocket.Message) error {
    // Convert WebSocket message to gRPC request
    req := &pb.ChatRequest{
        UserId:  client.GetUserID(),
        Content: msg.Data,
    }
    
    // Call gRPC service
    resp, err := b.grpcClient.SendMessage(context.Background(), req)
    if err != nil {
        return err
    }
    
    // Send gRPC response back to client
    return client.Send(resp.Data)
}
```

### 5. Custom Logger Adapters

#### Zap Logger
```go
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
```

#### Logrus Logger
```go
type LogrusLogger struct {
    logger *logrus.Logger
}

func (l *LogrusLogger) Debug(msg string, args ...any) {
    l.logger.WithFields(argsToFields(args)).Debug(msg)
}

func (l *LogrusLogger) Info(msg string, args ...any) {
    l.logger.WithFields(argsToFields(args)).Info(msg)
}

func (l *LogrusLogger) Warn(msg string, args ...any) {
    l.logger.WithFields(argsToFields(args)).Warn(msg)
}

func (l *LogrusLogger) Error(msg string, args ...any) {
    l.logger.WithFields(argsToFields(args)).Error(msg)
}

func argsToFields(args []any) logrus.Fields {
    fields := logrus.Fields{}
    for i := 0; i < len(args)-1; i += 2 {
        if key, ok := args[i].(string); ok {
            fields[key] = args[i+1]
        }
    }
    return fields
}
```

#### Slog Logger (Go 1.21+)
```go
type SlogLogger struct {
    logger *slog.Logger
}

func (l *SlogLogger) Debug(msg string, args ...any) {
    l.logger.Debug(msg, args...)
}

func (l *SlogLogger) Info(msg string, args ...any) {
    l.logger.Info(msg, args...)
}

func (l *SlogLogger) Warn(msg string, args ...any) {
    l.logger.Warn(msg, args...)
}

func (l *SlogLogger) Error(msg string, args ...any) {
    l.logger.Error(msg, args...)
}
```

### 6. Custom Metrics Adapters

#### Prometheus
```go
import "github.com/prometheus/client_golang/prometheus"

type PrometheusMetrics struct {
    connectionsGauge   prometheus.Gauge
    messagesCounter    prometheus.Counter
    messageSizeHist    prometheus.Histogram
    latencyHist        prometheus.Histogram
    errorsCounter      *prometheus.CounterVec
    roomJoinsCounter   prometheus.Counter
    roomLeavesCounter  prometheus.Counter
}

func NewPrometheusMetrics(reg prometheus.Registerer) *PrometheusMetrics {
    m := &PrometheusMetrics{
        connectionsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "websocket_connections_total",
            Help: "Current number of WebSocket connections",
        }),
        messagesCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "websocket_messages_total",
            Help: "Total number of messages processed",
        }),
        messageSizeHist: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name:    "websocket_message_size_bytes",
            Help:    "Size of WebSocket messages in bytes",
            Buckets: prometheus.ExponentialBuckets(100, 10, 5),
        }),
        latencyHist: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name:    "websocket_message_latency_seconds",
            Help:    "Message processing latency",
            Buckets: prometheus.DefBuckets,
        }),
        errorsCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "websocket_errors_total",
                Help: "Total number of errors",
            },
            []string{"type"},
        ),
        roomJoinsCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "websocket_room_joins_total",
            Help: "Total number of room joins",
        }),
        roomLeavesCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "websocket_room_leaves_total",
            Help: "Total number of room leaves",
        }),
    }
    
    reg.MustRegister(
        m.connectionsGauge,
        m.messagesCounter,
        m.messageSizeHist,
        m.latencyHist,
        m.errorsCounter,
        m.roomJoinsCounter,
        m.roomLeavesCounter,
    )
    
    return m
}

func (m *PrometheusMetrics) IncrementConnections() {
    m.connectionsGauge.Inc()
}

func (m *PrometheusMetrics) DecrementConnections() {
    m.connectionsGauge.Dec()
}

func (m *PrometheusMetrics) IncrementMessages() {
    m.messagesCounter.Inc()
}

func (m *PrometheusMetrics) RecordMessageSize(size int) {
    m.messageSizeHist.Observe(float64(size))
}

func (m *PrometheusMetrics) RecordLatency(duration time.Duration) {
    m.latencyHist.Observe(duration.Seconds())
}

func (m *PrometheusMetrics) IncrementErrors(errorType string) {
    m.errorsCounter.WithLabelValues(errorType).Inc()
}

func (m *PrometheusMetrics) IncrementRoomJoins() {
    m.roomJoinsCounter.Inc()
}

func (m *PrometheusMetrics) IncrementRoomLeaves() {
    m.roomLeavesCounter.Inc()
}
```

### 7. Rate Limiting Middleware

```go
import (
    "sync"
    "time"
    
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     r,
        burst:    b,
    }
}

func (rl *RateLimiter) Allow(clientID string) bool {
    rl.mu.RLock()
    limiter, exists := rl.limiters[clientID]
    rl.mu.RUnlock()
    
    if !exists {
        rl.mu.Lock()
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[clientID] = limiter
        rl.mu.Unlock()
    }
    
    return limiter.Allow()
}

func RateLimitMiddleware(limiter *RateLimiter) websocket.Middleware {
    return func(next websocket.HandlerFunc) websocket.HandlerFunc {
        return func(client *websocket.Client, msg *websocket.Message) error {
            if !limiter.Allow(client.ID) {
                return websocket.ErrRateLimitExceeded
            }
            return next(client, msg)
        }
    }
}
```

### 8. Complete Production Setup

```go
package main

import (
    "context"
    "net/http"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.uber.org/zap"
    "golang.org/x/time/rate"
    
    "github.com/yourusername/websocket"
    "yourapp/internal/auth"
    "yourapp/internal/database"
)

func main() {
    // Initialize logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    
    // Initialize metrics
    reg := prometheus.NewRegistry()
    metrics := NewPrometheusMetrics(reg)
    
    // Initialize database
    db := database.Connect()
    defer db.Close()
    
    // Initialize auth service
    authService := auth.NewService(db)
    
    // Create WebSocket hub
    config := websocket.DefaultConfig().
        WithMaxMessageSize(1024 * 1024).
        WithCompression(true).
        WithCheckOrigin(corsChecker())
    
    hub := websocket.NewHub(config)
    hub.SetLogger(&ZapLogger{logger})
    hub.SetMetrics(metrics)
    
    // Set limits
    limits := websocket.DefaultLimits().
        WithMaxConnections(10000).
        WithMaxConnectionsPerUser(5).
        WithMaxRoomsPerClient(10)
    hub.SetLimits(limits)
    
    // Set hooks
    hub.SetHooks(websocket.Hooks{
        BeforeConnect: func(r *http.Request) error {
            return authService.ValidateWebSocketToken(r)
        },
        AfterConnect: func(client *websocket.Client) {
            logger.Info("Client connected", zap.String("id", client.ID))
        },
    })
    
    // Create middleware chain
    limiter := NewRateLimiter(rate.Limit(100), 10)
    chain := websocket.NewMiddlewareChain(handleMessage).
        Use(websocket.RecoveryMiddleware(&ZapLogger{logger})).
        Use(websocket.LoggingMiddleware(&ZapLogger{logger})).
        Use(websocket.MetricsMiddleware(metrics)).
        Use(RateLimitMiddleware(limiter))
    
    hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
        return chain.Execute(client, msg)
    })
    
    // Start hub
    go hub.Run()
    
    // HTTP routes
    http.HandleFunc("/ws", hub.HandleHTTP())
    http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
    http.HandleFunc("/health", healthCheck)
    
    // Start server
    server := &http.Server{
        Addr:         ":8080",
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    logger.Info("Server starting", zap.String("addr", server.Addr))
    if err := server.ListenAndServe(); err != nil {
        logger.Fatal("Server failed", zap.Error(err))
    }
}

func corsChecker() func(*http.Request) bool {
    allowedOrigins := map[string]bool{
        "https://example.com":     true,
        "https://app.example.com": true,
    }
    
    return func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        return allowedOrigins[origin]
    }
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

## Testing Your Integration

```go
package websocket_test

import (
    "testing"
    "net/http/httptest"
    
    "github.com/gorilla/websocket"
    yourws "github.com/yourusername/websocket"
)

func TestWebSocketIntegration(t *testing.T) {
    // Create hub
    hub := yourws.NewHub(yourws.DefaultConfig())
    go hub.Run()
    defer hub.Shutdown(context.Background())
    
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(hub.HandleHTTP()))
    defer server.Close()
    
    // Connect client
    url := "ws" + server.URL[4:] + "/ws"
    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close()
    
    // Send message
    if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
        t.Fatal(err)
    }
    
    // Read response
    _, msg, err := conn.ReadMessage()
    if err != nil {
        t.Fatal(err)
    }
    
    t.Logf("Received: %s", msg)
}
```