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
package wshub
