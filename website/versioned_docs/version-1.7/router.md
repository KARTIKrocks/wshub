---
id: router
title: Router
description: Dispatch incoming messages to per-event handlers based on an event name extracted from each message.
---

# Router

Dispatch incoming messages to per-event handlers based on an event name
extracted from each message. The router is format-agnostic — JSON, msgpack,
binary, or anything else.

```go
import "github.com/KARTIKrocks/wshub"
```

- Event-based message dispatching
- Format-agnostic extractor function
- Chainable handler registration
- Fallback handler for unmatched events

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Router).

## Creating a Router

Create a router with an extractor function that determines the event name from
each message:

```go
router := wshub.NewRouter(func(msg *wshub.Message) string {
    var env struct{ Type string `json:"type"` }
    json.Unmarshal(msg.Data, &env)
    return env.Type
})

// Register handlers and use with a hub
hub := wshub.NewHub(wshub.WithMessageHandler(router.Handle))
```

## Registering Handlers

Use `On()` to register handlers for specific events. Calls can be chained:

| Method | Description |
| --- | --- |
| `On(event, handler)` | Register a handler for the given event name |
| `OnNotFound(handler)` | Set a fallback handler for unmatched events (defaults to returning `ErrInvalidMessage`) |
| `Handle(client, msg)` | Dispatch a message to the appropriate handler — pass to `WithMessageHandler` |

```go
router.
    On("chat",  handleChat).
    On("join",  handleJoin).
    On("leave", handleLeave).
    OnNotFound(func(client *wshub.Client, msg *wshub.Message) error {
        return client.SendText("unknown event")
    })
```

## Full Example

A complete example using the router with rooms for a chat application:

```go
func main() {
    router := wshub.NewRouter(func(msg *wshub.Message) string {
        var env struct{ Type string `json:"type"` }
        json.Unmarshal(msg.Data, &env)
        return env.Type
    })

    router.
        On("chat", func(c *wshub.Client, msg *wshub.Message) error {
            return c.Hub().BroadcastText(msg.Text(), nil)
        }).
        On("join", func(c *wshub.Client, msg *wshub.Message) error {
            var req struct{ Room string `json:"room"` }
            json.Unmarshal(msg.Data, &req)
            return c.Hub().JoinRoom(c, req.Room)
        }).
        On("leave", func(c *wshub.Client, msg *wshub.Message) error {
            var req struct{ Room string `json:"room"` }
            json.Unmarshal(msg.Data, &req)
            return c.Hub().LeaveRoom(c, req.Room)
        })

    hub := wshub.NewHub(
        wshub.WithMessageHandler(router.Handle),
    )

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    go hub.Run(ctx)
}
```
