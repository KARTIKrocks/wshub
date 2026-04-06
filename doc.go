// Package wshub provides production-ready WebSocket connection management
// with support for rooms, broadcasting, middleware, hooks, and extensibility.
//
// The central type is [Hub], which manages all connected clients and provides
// broadcasting, room management, and lifecycle hooks. Create one with [NewHub]
// and configure it using functional options:
//
//	hub := wshub.NewHub(
//	    wshub.WithConfig(wshub.DefaultConfig().WithCompression(true)),
//	    wshub.WithLogger(myLogger),
//	    wshub.WithLimits(wshub.DefaultLimits().WithMaxConnections(10000)),
//	    wshub.WithHooks(wshub.Hooks{
//	        AfterConnect: func(c *wshub.Client) { /* ... */ },
//	    }),
//	    wshub.WithMessageHandler(handler),
//	)
//	go hub.Run()
//
// # Connection Lifecycle
//
// Upgrade HTTP connections with [Hub.UpgradeConnection] or the convenience
// [Hub.HandleHTTP] handler. Each connection spawns a read pump and write pump
// goroutine managed by the hub. Use [Hub.Shutdown] for graceful teardown.
//
// # Graceful Draining
//
// For zero-downtime rolling deploys (e.g. Kubernetes), call [Hub.Drain]
// before [Hub.Shutdown]. Drain stops accepting new connections (returning
// HTTP 503) while letting existing connections finish their in-flight
// messages. Idle connections are proactively closed after the configured
// drain timeout (see [WithDrainTimeout]). Inspect the hub's lifecycle
// state with [Hub.State], [Hub.IsRunning], and [Hub.IsDraining] to
// implement health and readiness probes:
//
//	// preStop / SIGTERM handler
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	hub.Drain(ctx)    // stop new connections, wait for existing ones
//	hub.Shutdown(ctx) // force-close anything remaining
//
// # Health Probes
//
// Use [Hub.HealthHandler] and [Hub.ReadyHandler] as HTTP handlers for
// liveness and readiness probes. For programmatic access, [Hub.Health]
// returns a [HealthStatus] snapshot, and [Hub.Alive] / [Hub.Ready]
// provide simple boolean checks:
//
//	http.Handle("/healthz", hub.HealthHandler())
//	http.Handle("/readyz", hub.ReadyHandler())
//
// # Rooms
//
// Clients can join and leave named rooms via [Hub.JoinRoom] and [Hub.LeaveRoom].
// Broadcast to a room with [Hub.BroadcastToRoom]. Rooms are created lazily and
// removed automatically when the last client leaves.
//
// # Middleware
//
// Use [MiddlewareChain] to compose message-processing pipelines. Built-in
// middlewares include [LoggingMiddleware], [RecoveryMiddleware], and
// [MetricsMiddleware]. Call [MiddlewareChain.Build] to pre-compute the chain
// for better performance.
//
// # Message Routing
//
// [Router] dispatches messages to per-event handlers based on an extractor
// function, decoupling routing from any specific message format.
//
// # Multi-Node Scaling
//
// Scale horizontally by setting an [Adapter] via [WithAdapter]. All broadcast
// and targeted send methods automatically relay messages to other nodes
// through the adapter's message bus. The core package ships no adapter
// implementations — import a subpackage to avoid pulling in unwanted
// dependencies:
//
//   - github.com/KARTIKrocks/wshub/adapter/redis — Redis Pub/Sub
//   - github.com/KARTIKrocks/wshub/adapter/nats  — NATS core Pub/Sub
//
// Implement the [Adapter] interface to integrate any other message bus.
//
// Enable [WithPresence] to exchange periodic heartbeats between nodes.
// [Hub.GlobalClientCount] and [Hub.GlobalRoomCount] then return cluster-wide
// totals. Nodes that miss three consecutive heartbeats are automatically
// evicted from the totals.
//
// # Backpressure
//
// When a client's send buffer is full, the hub applies the configured
// [DropPolicy] (set via [WithDropPolicy]):
//
//   - [DropNewest] (default) discards the new message.
//   - [DropOldest] evicts the oldest queued message to make room.
//
// In both cases the [Hooks.OnSendDropped] callback fires so the application
// can log, disconnect slow clients, or take other corrective action.
//
// # Write Coalescing
//
// When [Config.CoalesceWrites] is true, the write pump batches queued text
// messages into a single WebSocket frame separated by newline bytes (\n).
// This reduces the number of syscalls under high throughput. Binary messages
// are always sent as individual frames. Receivers must split coalesced
// frames on \n to recover individual messages.
//
//	cfg := wshub.DefaultConfig().WithCoalesceWrites(true)
//	hub := wshub.NewHub(wshub.WithConfig(cfg))
package wshub
