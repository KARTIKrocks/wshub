---
id: hooks
title: Hooks
description: Lifecycle hooks that run custom logic at key connection, message, and room events.
---

# Hooks

Lifecycle hooks let you run custom logic at key connection, message, and room
events.

```go
import "github.com/KARTIKrocks/wshub"
```

- Connection lifecycle: `BeforeConnect`, `AfterConnect`, `BeforeDisconnect`, `AfterDisconnect`
- Message lifecycle: `BeforeMessage`, `AfterMessage`
- Room lifecycle: `BeforeRoomJoin`, `AfterRoomJoin`, `BeforeRoomLeave`, `AfterRoomLeave`
- Error handling hook: `OnError`
- Backpressure hook: `OnSendDropped`
- Before-hooks can reject operations by returning an error
- `BeforeDisconnect` runs with a configurable timeout (default: 5s)

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Hooks).

## Connection Hooks

| Hook | Description |
| --- | --- |
| `BeforeConnect(r *http.Request) error` | Called before upgrading — return an error to reject |
| `AfterConnect(client *Client)` | Called after a client connects |
| `BeforeDisconnect(client *Client)` | Called before disconnect (runs with a timeout, default 5s via `WithHookTimeout`) |
| `AfterDisconnect(client *Client)` | Called after a client disconnects |
| `OnError(client *Client, err error)` | Called on client errors |
| `OnSendDropped(client *Client, data []byte)` | Called when a message is dropped due to a full send buffer |

```go
hub := wshub.NewHub(
    wshub.WithHooks(wshub.Hooks{
        BeforeConnect: func(r *http.Request) error {
            // Validate auth token before upgrade
            token := r.Header.Get("Authorization")
            if !validateToken(token) {
                return wshub.ErrAuthenticationFailed
            }
            return nil
        },
        AfterConnect: func(client *wshub.Client) {
            // Set user ID from auth context
            userID := extractUserID(client.Request())
            client.SetUserID(userID)
            log.Printf("User %s connected (client: %s)", userID, client.ID)
        },
        AfterDisconnect: func(client *wshub.Client) {
            log.Printf("Client %s disconnected", client.ID)
        },
        OnError: func(client *wshub.Client, err error) {
            log.Printf("Error for %s: %v", client.ID, err)
        },
        OnSendDropped: func(client *wshub.Client, data []byte) {
            // Called when a message is dropped because the send buffer is full.
            // Keep this fast — it runs in the sender's goroutine.
            log.Printf("Dropped %d bytes for slow client %s", len(data), client.ID)
        },
    }),
    // Configure the BeforeDisconnect timeout (default: 5s)
    wshub.WithHookTimeout(10*time.Second),
)
```

## Message Hooks

| Hook | Description |
| --- | --- |
| `BeforeMessage(client, msg) (*Message, error)` | Called before processing — can modify or reject the message |
| `AfterMessage(client, msg, err)` | Called after message processing completes |

```go
wshub.WithHooks(wshub.Hooks{
    BeforeMessage: func(client *wshub.Client, msg *wshub.Message) (*wshub.Message, error) {
        // Reject empty messages
        if len(msg.Data) == 0 {
            return nil, fmt.Errorf("empty message")
        }
        // Modify the message (e.g., sanitize)
        return msg, nil
    },
    AfterMessage: func(client *wshub.Client, msg *wshub.Message, err error) {
        if err != nil {
            log.Printf("Message handling failed: %v", err)
        }
    },
})
```

## Room Hooks

| Hook | Description |
| --- | --- |
| `BeforeRoomJoin(client, room) error` | Called before joining — return an error to reject |
| `AfterRoomJoin(client, room)` | Called after joining a room |
| `BeforeRoomLeave(client, room)` | Called before leaving a room |
| `AfterRoomLeave(client, room)` | Called after leaving a room |

```go
wshub.WithHooks(wshub.Hooks{
    BeforeRoomJoin: func(client *wshub.Client, room string) error {
        // Check if client is authorized to join this room
        if !isAuthorized(client, room) {
            return wshub.ErrUnauthorized
        }
        return nil
    },
    AfterRoomJoin: func(client *wshub.Client, room string) {
        // Notify room members
        hub.BroadcastToRoomExcept(room, []byte(client.ID+" joined"), client)
    },
    AfterRoomLeave: func(client *wshub.Client, room string) {
        hub.BroadcastToRoom(room, []byte(client.ID+" left"))
    },
})
```
