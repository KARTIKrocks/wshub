package wshub

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

// WithMessageHandler sets the hub-level message handler.
func WithMessageHandler(fn func(*Client, *Message) error) Option {
	return func(h *Hub) {
		h.onMessage = fn
	}
}
