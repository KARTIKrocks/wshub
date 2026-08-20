---
id: client
title: Client
description: A single WebSocket connection, with methods for sending messages, managing metadata, and handling events.
---

# Client

Represents a single WebSocket connection with methods for sending messages,
managing metadata, and handling events.

```go
import "github.com/KARTIKrocks/wshub"
```

- Unique UUID-based client identification
- User ID support for multi-device connections
- Arbitrary per-client metadata storage
- Multiple message sending formats (text, binary, JSON)
- Per-client event callbacks (`OnMessage`, `OnClose`, `OnError`)
- Access to the original HTTP request

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Client).

## Properties

| Property / Method | Description |
| --- | --- |
| `client.ID` | Unique client identifier (UUID) |
| `SetUserID(userID)` | Set user ID for multi-device support |
| `GetUserID()` | Get the assigned user ID |
| `ConnectedAt()` | Connection timestamp |
| `IsClosed()` | Whether the connection is closed |
| `ClosedAt()` | When the connection was closed |
| `Request()` | Access the original HTTP request |

```go
// Access client properties
log.Printf("Client ID: %s", client.ID)
log.Printf("Connected at: %v", client.ConnectedAt())

// Multi-device user identification
client.SetUserID("user-456")
userID := client.GetUserID()

// Access original HTTP request (for auth headers, cookies, etc.)
req := client.Request()
token := req.Header.Get("Authorization")
```

## Sending Messages

| Method | Description |
| --- | --- |
| `Send(data)` | Send raw bytes |
| `SendText(text)` | Send a text string |
| `SendJSON(v)` | JSON-encode and send |
| `SendRawJSON(data)` | Send pre-serialized JSON bytes (0 allocs, skips marshaling) |
| `SendBinary(data)` | Send a binary message |
| `SendMessage(msgType, data)` | Send with a specific message type (applies drop policy) |
| `SendWithContext(ctx, data)` | Send text with context support (blocks until enqueued) |
| `SendMessageWithContext(ctx, msgType, data)` | Send with type and context (blocks until enqueued) |

```go
// Send different message types
client.Send([]byte("raw bytes"))
client.SendText("hello")
client.SendBinary(binaryData)

// Send JSON
client.SendJSON(map[string]any{
    "type": "chat",
    "text": "hello world",
    "time": time.Now(),
})

// Send with context (blocks until enqueued, unlike SendMessage which applies drop policy)
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
client.SendWithContext(ctx, []byte("important message"))

// Send with specific type and context
client.SendMessageWithContext(ctx, wshub.BinaryMessage, binaryData)
```

## Metadata

Store arbitrary per-client data using the metadata API:

```go
// Store request-scoped data on the client
client.SetMetadata("role", "admin")
client.SetMetadata("display_name", "Alice")

// Retrieve metadata
role, ok := client.GetMetadata("role")
if ok {
    log.Printf("Role: %v", role)
}

// Remove metadata
client.DeleteMetadata("temporary_key")
```

## Callbacks

Register per-client event handlers:

```go
client.OnMessage(func(c *wshub.Client, msg *wshub.Message) {
    // Handle messages for this specific client
    log.Printf("Message: %s", msg.Text())
})

client.OnClose(func(c *wshub.Client) {
    // Clean up when this client disconnects
    log.Printf("Client %s disconnected", c.ID)
})

client.OnError(func(c *wshub.Client, err error) {
    // Handle errors for this client
    log.Printf("Error for %s: %v", c.ID, err)
})
```

## Closing

```go
// Close with default close code
client.Close()

// Close with a specific WebSocket close code and reason
client.CloseWithCode(websocket.CloseNormalClosure, "goodbye")
```
