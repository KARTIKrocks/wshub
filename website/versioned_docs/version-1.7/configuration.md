---
id: configuration
title: Configuration
description: WebSocket configuration using the builder pattern — buffer sizes, timeouts, compression, and origin checking.
---

# Configuration

Extensive WebSocket configuration using the builder pattern for buffer sizes,
timeouts, compression, and origin checking.

```go
import "github.com/KARTIKrocks/wshub"
```

- Sensible defaults out of the box
- Builder pattern for fluent configuration
- Configurable buffer sizes, timeouts, and message limits
- Per-message compression support
- Opt-in write coalescing for high-throughput text broadcasts
- Same-origin checking on by default

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#Config).

## Default Config

| Option | Default |
| --- | --- |
| `ReadBufferSize` | 1024 |
| `WriteBufferSize` | 1024 |
| `WriteWait` | 10s |
| `PongWait` | 60s |
| `PingPeriod` | 54s (90% of `PongWait`) |
| `MaxMessageSize` | 512 KB |
| `SendChannelSize` | 256 |
| `EnableCompression` | false |
| `CoalesceWrites` | false |
| `CheckOrigin` | `AllowSameOrigin` |

```go
// Use default config
config := wshub.DefaultConfig()

// Use with hub
hub := wshub.NewHub(
    wshub.WithConfig(config),
)
```

## Builder Methods

| Method | Description |
| --- | --- |
| `WithBufferSizes(read, write)` | Set read and write buffer sizes |
| `WithMaxMessageSize(size)` | Set maximum message size in bytes |
| `WithCompression(enabled)` | Enable per-message compression |
| `WithCoalesceWrites(enabled)` | Batch queued text messages into a single WebSocket frame (newline-separated), reducing syscalls under high throughput |
| `WithCheckOrigin(fn)` | Set the origin validation function |

```go
config := wshub.DefaultConfig().
    WithBufferSizes(4096, 4096).
    WithMaxMessageSize(1024 * 1024). // 1 MB
    WithCompression(true).
    WithCoalesceWrites(true). // batch text messages into single frames
    WithCheckOrigin(wshub.AllowOrigins("https://example.com"))

hub := wshub.NewHub(
    wshub.WithConfig(config),
)
```

## Origin Checking

Since v1.7.0 the default is `AllowSameOrigin`. Earlier versions defaulted to
`AllowAllOrigins`, which let any page on any site open an authenticated
connection using the visitor's cookies (cross-site WebSocket hijacking).

If your front-end is served from a different origin than the WebSocket
endpoint, allowlist it with `AllowOrigins` — otherwise those upgrades are
rejected with `403` and an `origin_rejected` metric.

| Function | Description |
| --- | --- |
| `AllowAllOrigins` | Allow connections from any origin — development only |
| `AllowSameOrigin` | Only allow same-origin connections (the default) |
| `AllowOrigins(origins...)` | Allow specific origins, compared as full origin strings |

```go
// Same-origin only (the default — no call needed)
config.WithCheckOrigin(wshub.AllowSameOrigin)

// Specific origins — use this when your front-end is served from a
// different host than the WebSocket endpoint
config.WithCheckOrigin(wshub.AllowOrigins(
    "https://example.com",
    "https://app.example.com",
))

// Custom checker
config.WithCheckOrigin(func(r *http.Request) bool {
    return strings.HasSuffix(r.Header.Get("Origin"), ".example.com")
})

// Disable the check entirely — development only
config.WithCheckOrigin(wshub.AllowAllOrigins)
```

Requests with no `Origin` header are allowed by both `AllowSameOrigin` and
`AllowOrigins`, since non-browser clients (mobile apps, CLI tools,
server-to-server) typically omit it. Browsers always send it, so the cross-site
hijacking path stays closed.

:::note Scheme is not compared

`AllowSameOrigin` compares host and port, not scheme, so `http://example.com` is
accepted by a server reachable at `example.com` over https. A server behind a
TLS-terminating proxy cannot see its own scheme, so comparing it would reject
the legitimate origins of every proxied deployment. Use `AllowOrigins` if you
need scheme-exact matching.

:::
