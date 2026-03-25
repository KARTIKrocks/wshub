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
package wshub
