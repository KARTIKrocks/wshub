# Quick Start Guide

## Get Started in 5 Minutes

### 1. Install Dependencies

```bash
go get github.com/KARTIKrocks/wshub
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

    "github.com/KARTIKrocks/wshub"
)

func main() {
    // Create hub with message handler
    hub := wshub.NewHub(
        wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
            log.Printf("Received: %s from %s", msg.Text(), client.ID)

            // Echo back to sender
            return client.Send(msg.Data)
        }),
    )

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

## Next Steps

### Add Rooms

```go
hub := wshub.NewHub(
    wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
        // Join a room
        hub.JoinRoom(client, "general")

        // Broadcast to room
        return hub.BroadcastToRoom("general", msg.Data)
    }),
)
```

### Add Authentication

```go
hub := wshub.NewHub(
    wshub.WithHooks(wshub.Hooks{
        BeforeConnect: func(r *http.Request) error {
            token := r.Header.Get("Authorization")
            if token != "valid-token" {
                return wshub.ErrAuthenticationFailed
            }
            return nil
        },
    }),
)
```

### Add Logging

```go
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, args ...any) {
    log.Printf("[DEBUG] %s %s", msg, formatArgs(args))
}
func (l *SimpleLogger) Info(msg string, args ...any) {
    log.Printf("[INFO] %s %s", msg, formatArgs(args))
}
func (l *SimpleLogger) Warn(msg string, args ...any) {
    log.Printf("[WARN] %s %s", msg, formatArgs(args))
}
func (l *SimpleLogger) Error(msg string, args ...any) {
    log.Printf("[ERROR] %s %s", msg, formatArgs(args))
}

func formatArgs(args []any) string {
    if len(args) == 0 {
        return ""
    }
    parts := make([]string, 0, len(args)/2)
    for i := 0; i+1 < len(args); i += 2 {
        parts = append(parts, fmt.Sprintf("%v=%v", args[i], args[i+1]))
    }
    return strings.Join(parts, " ")
}

// Use it
hub := wshub.NewHub(wshub.WithLogger(&SimpleLogger{}))
```

### Add Rate Limiting

```go
limits := wshub.DefaultLimits().
    WithMaxMessageRate(100)

hub := wshub.NewHub(wshub.WithLimits(limits))
```

## Common Patterns

### JSON Messages

```go
type Message struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

hub := wshub.NewHub(
    wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
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
            return wshub.ErrInvalidMessage
        }

        return nil
    }),
)
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

## Troubleshooting

### Connection Rejected

```go
// Check your origin checker
config := wshub.DefaultConfig().
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

## Best Practices

1. Always use graceful shutdown
2. Set appropriate timeouts
3. Implement error handling
4. Use middleware for cross-cutting concerns
5. Log important events
6. Set connection limits
7. Validate message size
8. Use TLS in production (wss://)
9. Implement rate limiting
10. Monitor metrics

## Need Help?

1. Check the examples in `examples/`
2. Read the full documentation in `README.md`
3. Look at the code comments
