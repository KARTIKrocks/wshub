---
id: messages
title: Messages
description: The Message type represents incoming WebSocket messages, with helpers for text and JSON.
---

# Messages

The `Message` type represents incoming WebSocket messages with helpers for
common formats.

```go
import "github.com/KARTIKrocks/wshub"
```

- Typed message representation (text and binary)
- Convenience helpers for text and JSON parsing
- Includes sender client ID and receive timestamp
- Pre-serialized JSON API for zero-alloc fan-out (~35 ns per send)

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Message).

## Message Type

| Field / Method | Description |
| --- | --- |
| `Type` | `MessageType` (`TextMessage` or `BinaryMessage`) |
| `Data` | Raw message data as `[]byte` |
| `ClientID` | Sender's client ID |
| `Time` | Receive timestamp |
| `Text()` | Data as a string |
| `JSON(v)` | Unmarshal data as JSON into `v` |

```go
type Message struct {
    Type     MessageType // TextMessage, BinaryMessage
    Data     []byte      // Raw message data
    ClientID string      // Sender's client ID
    Time     time.Time   // Receive timestamp
}

// Convenience helpers
text := msg.Text()         // Data as string

var payload ChatMessage
err := msg.JSON(&payload)  // Unmarshal as JSON
```

## Message Handler

Set a message handler when creating the hub to process incoming messages:

```go
hub := wshub.NewHub(
    wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
        // Parse the incoming message
        var chatMsg struct {
            Room string `json:"room"`
            Text string `json:"text"`
        }
        if err := msg.JSON(&chatMsg); err != nil {
            return err
        }

        // Broadcast to a room
        response, _ := json.Marshal(map[string]string{
            "from": client.ID,
            "text": chatMsg.Text,
        })
        hub.BroadcastToRoom(chatMsg.Room, response)
        return nil
    }),
)
```

## Pre-serialized JSON

When you marshal JSON once and fan it out to many clients, use the raw JSON API
to skip re-serialization entirely. This is ideal for high-throughput broadcast
patterns where the same payload goes to hundreds or thousands of connections.

| Function / Method | Description |
| --- | --- |
| `NewRawJSONMessage(data)` | Create a message from already-marshaled JSON bytes |
| `Hub.BroadcastRawJSON(data)` | Broadcast pre-serialized JSON to all clients |
| `Client.SendRawJSON(data)` | Send pre-serialized JSON to a single client |

```go
// Marshal once, broadcast to all — 0 allocs per send (~35 ns vs ~1,000 ns)
data, _ := json.Marshal(map[string]any{
    "type":    "position",
    "x":       player.X,
    "y":       player.Y,
    "playerID": player.ID,
})

// Fan out pre-serialized bytes (no per-client json.Marshal)
hub.BroadcastRawJSON(data)

// Or send to a single client
client.SendRawJSON(data)
```
