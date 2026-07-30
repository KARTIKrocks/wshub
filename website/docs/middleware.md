---
id: middleware
title: Middleware
description: Chain message handlers with cross-cutting logic using the middleware pattern.
---

# Middleware

Chain message handlers with custom logic using the middleware pattern.
Middleware wraps the message handler to add cross-cutting concerns.

```go
import "github.com/KARTIKrocks/wshub"
```

- Composable middleware chain
- Built-in logging, recovery, and metrics middleware
- Easy to write custom middleware

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Middleware).

## Middleware Chain

```go
type Middleware func(HandlerFunc) HandlerFunc
type HandlerFunc func(*Client, *Message) error

// Build a middleware chain
chain := wshub.NewMiddlewareChain(finalHandler).
    Use(wshub.RecoveryMiddleware(logger)).
    Use(wshub.LoggingMiddleware(logger)).
    Use(wshub.MetricsMiddleware(metrics)).
    Build()

// Use with the hub
hub := wshub.NewHub(
    wshub.WithMessageHandler(chain),
)
```

## Built-in Middleware

| Middleware | Description |
| --- | --- |
| `LoggingMiddleware(logger)` | Log message events |
| `RecoveryMiddleware(logger)` | Recover from panics in message handlers |
| `MetricsMiddleware(metrics)` | Record message processing metrics |

```go
// Use built-in middleware
chain := wshub.NewMiddlewareChain(handler).
    Use(wshub.RecoveryMiddleware(logger)).   // catch panics
    Use(wshub.LoggingMiddleware(logger)).    // log messages
    Use(wshub.MetricsMiddleware(metrics)).   // record metrics
    Build()
```

:::tip

When using `MetricsMiddleware`, pass `wshub.WithoutHandlerLatency()` to the hub
so handler latency is not recorded twice.

:::

## Custom Middleware

Write custom middleware by implementing the `Middleware` signature:

```go
// Custom middleware that filters messages
func ProfanityFilter(next wshub.HandlerFunc) wshub.HandlerFunc {
    return func(client *wshub.Client, msg *wshub.Message) error {
        if containsProfanity(msg.Text()) {
            client.SendText("Message blocked: inappropriate content")
            return nil // swallow the message
        }
        return next(client, msg)
    }
}

// Custom middleware that adds timing
func TimingMiddleware(next wshub.HandlerFunc) wshub.HandlerFunc {
    return func(client *wshub.Client, msg *wshub.Message) error {
        start := time.Now()
        err := next(client, msg)
        log.Printf("Message processed in %v", time.Since(start))
        return err
    }
}

// Use in chain
chain := wshub.NewMiddlewareChain(handler).
    Use(ProfanityFilter).
    Use(TimingMiddleware).
    Build()
```
