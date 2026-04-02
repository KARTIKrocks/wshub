package wshub

import "time"

// Option configures a Hub during construction.
type Option func(*Hub)

// WithConfig sets the WebSocket configuration.
func WithConfig(config Config) Option {
	return func(h *Hub) {
		h.config = config
	}
}

// WithLogger sets the logger for the hub.
func WithLogger(logger Logger) Option {
	return func(h *Hub) {
		h.logger = logger
	}
}

// WithMetrics sets the metrics collector for the hub.
func WithMetrics(metrics MetricsCollector) Option {
	return func(h *Hub) {
		h.metrics = metrics
	}
}

// WithLimits sets the limits for the hub.
func WithLimits(limits Limits) Option {
	return func(h *Hub) {
		h.limits = limits
	}
}

// WithHooks sets the lifecycle hooks for the hub.
func WithHooks(hooks Hooks) Option {
	return func(h *Hub) {
		h.hooks = hooks
	}
}

// WithParallelBroadcast enables parallel broadcasting with the given batch size.
// batchSize determines how many clients each goroutine handles (recommended: 50-200).
func WithParallelBroadcast(batchSize int) Option {
	return func(h *Hub) {
		h.useParallel = true
		if batchSize > 0 {
			h.parallelBatchSize = batchSize
		}
	}
}

// WithParallelBroadcastWorkers sets the number of persistent worker goroutines
// used for parallel broadcasting. The default is runtime.NumCPU().
// This option has no effect unless WithParallelBroadcast is also set.
func WithParallelBroadcastWorkers(n int) Option {
	return func(h *Hub) {
		if n > 0 {
			h.poolSize = n
		}
	}
}

// WithMessageHandler sets the hub-level message handler.
func WithMessageHandler(fn func(*Client, *Message) error) Option {
	return func(h *Hub) {
		h.onMessage = fn
	}
}

// WithoutHandlerLatency disables the hub's automatic handler latency
// recording. Use this when your message handler chain already includes
// MetricsMiddleware to avoid double-counting latency.
func WithoutHandlerLatency() Option {
	return func(h *Hub) {
		h.skipHandlerLatency = true
	}
}

// WithDropPolicy sets the behavior when a client's send buffer is full.
// The default is DropNewest which discards the new message. DropOldest
// evicts the oldest queued message to make room for the new one.
func WithDropPolicy(policy DropPolicy) Option {
	return func(h *Hub) {
		h.dropPolicy = policy
	}
}

// WithNodeID sets a deterministic node identifier for this hub. By default a
// random UUID is generated. Setting a stable ID (e.g., hostname or pod name)
// makes logs and debugging easier in multi-node deployments.
func WithNodeID(id string) Option {
	return func(h *Hub) {
		if id != "" {
			h.nodeID = id
		}
	}
}

// WithHookTimeout sets the maximum time to wait for synchronous lifecycle
// hooks (e.g. BeforeDisconnect) before proceeding. Default: 5s.
func WithHookTimeout(timeout time.Duration) Option {
	return func(h *Hub) {
		if timeout > 0 {
			h.hookTimeout = timeout
		}
	}
}

// WithAdapter sets the multi-node adapter for cross-node message delivery.
// When configured, every broadcast and targeted send is relayed to other
// nodes through the adapter, enabling horizontal scaling behind a load
// balancer.
func WithAdapter(adapter Adapter) Option {
	return func(h *Hub) {
		h.adapter = adapter
	}
}

// WithDrainTimeout sets the maximum time an idle connection can remain open
// after [Hub.Drain] is called. Connections whose send buffers have been empty
// for this duration are proactively closed with CloseGoingAway (1001).
//
// Default: 30s. Set to 0 to disable the idle connection reaper entirely,
// relying solely on natural client disconnection during drain.
func WithDrainTimeout(timeout time.Duration) Option {
	return func(h *Hub) {
		if timeout >= 0 {
			h.drainTimeout = timeout
		}
	}
}

// WithPresence enables periodic presence broadcasting for multi-node stats.
// Each hub publishes its local client and room counts at the given interval,
// allowing GlobalClientCount and GlobalRoomCount to return cluster-wide totals.
//
// When interval is zero, the default of 5 seconds is used. Nodes that miss
// 3 consecutive heartbeats are considered stale and evicted from the totals.
//
// Presence requires an adapter to be set via WithAdapter; without one it is a
// no-op.
func WithPresence(interval time.Duration) Option {
	return func(h *Hub) {
		if interval <= 0 {
			interval = 5 * time.Second
		}
		h.presenceInterval = interval
		h.presenceTTL = 3 * interval
		// presenceCache is allocated lazily in Run() only when an adapter
		// is also configured, avoiding a pointless allocation otherwise.
	}
}
