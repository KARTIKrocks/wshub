# Quick Start Guide

## 🚀 Get Started in 5 Minutes

### 1. Install Dependencies

```bash
go get github.com/google/uuid
go get github.com/gorilla/websocket
```

### 2. Create a Basic Server

Create `main.go`:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "yourproject/pkg/websocket"
)

func main() {
    // Create hub
    hub := websocket.NewHub(websocket.DefaultConfig())

    // Handle messages
    hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
        log.Printf("Received: %s from %s", msg.Text(), client.ID)

        // Echo back to sender
        return client.Send(msg.Data)
    })

    // Start hub
    go hub.Run()

    // Setup HTTP
    http.HandleFunc("/ws", hub.HandleHTTP())

    log.Println("Server running on :8080")
    http.ListenAndServe(":8080", nil)
}
```

### 3. Test with JavaScript Client

Create `index.html`:

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

### 4. Run and Test

```bash
go run main.go
```

Open `http://localhost:8080` in your browser and start chatting!

## 📚 Next Steps

### Add Rooms

```go
hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
    // Join a room
    hub.JoinRoom(client, "general")

    // Broadcast to room
    return hub.BroadcastToRoom("general", msg.Data)
})
```

### Add Authentication

```go
hub.SetHooks(websocket.Hooks{
    BeforeConnect: func(r *http.Request) error {
        token := r.Header.Get("Authorization")
        if token != "valid-token" {
            return websocket.ErrAuthenticationFailed
        }
        return nil
    },
})
```

### Add Logging

```go
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, args ...any) {
    log.Printf("[DEBUG] "+msg, args...)
}
func (l *SimpleLogger) Info(msg string, args ...any) {
    log.Printf("[INFO] "+msg, args...)
}
func (l *SimpleLogger) Warn(msg string, args ...any) {
    log.Printf("[WARN] "+msg, args...)
}
func (l *SimpleLogger) Error(msg string, args ...any) {
    log.Printf("[ERROR] "+msg, args...)
}

// Use it
hub.SetLogger(&SimpleLogger{})
```

### Add Rate Limiting

```go
type SimpleRateLimiter struct {
    requests map[string]int
    mu       sync.Mutex
}

func (r *SimpleRateLimiter) Allow(clientID string) bool {
    r.mu.Lock()
    defer r.mu.Unlock()

    count := r.requests[clientID]
    if count >= 100 { // 100 messages per client
        return false
    }
    r.requests[clientID] = count + 1
    return true
}

func RateLimitMiddleware(limiter *SimpleRateLimiter) websocket.Middleware {
    return func(next websocket.HandlerFunc) websocket.HandlerFunc {
        return func(client *websocket.Client, msg *websocket.Message) error {
            if !limiter.Allow(client.ID) {
                return websocket.ErrRateLimitExceeded
            }
            return next(client, msg)
        }
    }
}

// Use it
limiter := &SimpleRateLimiter{requests: make(map[string]int)}
chain := websocket.NewMiddlewareChain(yourHandler).
    Use(RateLimitMiddleware(limiter))

hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
    return chain.Execute(client, msg)
})
```

## 🎓 Learn More

- **Full Documentation**: See `README.md`
- **Integration Guide**: See `USAGE.md`
- **Implementation Guide**: See `IMPLEMENTATION_GUIDE.md`
- **Examples**: Check the `examples/` directory

## 💡 Common Patterns

### JSON Messages

```go
type Message struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
    var message Message
    if err := json.Unmarshal(msg.Data, &message); err != nil {
        return err
    }

    switch message.Type {
    case "chat":
        // Handle chat message
    case "join":
        // Handle join request
    default:
        return websocket.ErrInvalidMessage
    }

    return nil
})
```

### Broadcasting

```go
// To all clients
hub.Broadcast(data)

// To all except one
hub.BroadcastExcept(data, sender)

// To a specific room
hub.BroadcastToRoom("general", data)

// To a specific user (all their connections)
hub.SendToUser(userID, data)
```

### User Management

```go
// After authentication
client.SetUserID("user-123")

// Store custom data
client.SetMetadata("username", "John")
client.SetMetadata("role", "admin")

// Retrieve
username, ok := client.GetMetadata("username")

// Check if user is online
client, ok := hub.GetClientByUserID("user-123")

// Get all connections for a user
clients := hub.GetClientsByUserID("user-123")
```

## 🐛 Troubleshooting

### Connection Rejected

```go
// Check your origin checker
config := websocket.DefaultConfig().
    WithCheckOrigin(func(r *http.Request) bool {
        log.Printf("Origin: %s", r.Header.Get("Origin"))
        return true // Allow all for testing
    })
```

### Messages Not Received

```go
// Check if client is registered
if hub.ClientCount() == 0 {
    log.Println("No clients connected")
}

// Check if message is sent
hub.OnMessage(func(client *websocket.Client, msg *websocket.Message) error {
    log.Printf("Message received from %s: %s", client.ID, msg.Text())
    return nil
})
```

### Server Not Shutting Down

```go
// Always use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := hub.Shutdown(ctx); err != nil {
    log.Printf("Shutdown timeout: %v", err)
}
```

## 🎯 Best Practices

1. ✅ Always use graceful shutdown
2. ✅ Set appropriate timeouts
3. ✅ Implement error handling
4. ✅ Use middleware for cross-cutting concerns
5. ✅ Log important events
6. ✅ Set connection limits
7. ✅ Validate message size
8. ✅ Use TLS in production (wss://)
9. ✅ Implement rate limiting
10. ✅ Monitor metrics

## 📞 Need Help?

1. Check the examples in `examples/`
2. Read the full documentation in `README.md`
3. Review integration patterns in `USAGE.md`
4. Look at the code comments

Happy coding! 🎉
