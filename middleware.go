package wshub

import (
	"sync"
	"time"
)

// HandlerFunc is a function that handles WebSocket messages.
type HandlerFunc func(*Client, *Message) error

// Middleware wraps a HandlerFunc to add additional functionality.
type Middleware func(HandlerFunc) HandlerFunc

// MiddlewareChain manages a chain of middlewares.
// All mutations (Use) must happen before the first Execute call.
// Execute is safe for concurrent use once the chain is built.
type MiddlewareChain struct {
	mu          sync.RWMutex
	middlewares []Middleware
	handler     HandlerFunc
	built       HandlerFunc // cached composed handler
}

// NewMiddlewareChain creates a new middleware chain with the final handler.
func NewMiddlewareChain(handler HandlerFunc) *MiddlewareChain {
	return &MiddlewareChain{
		handler:     handler,
		middlewares: make([]Middleware, 0),
	}
}

// Use adds a middleware to the chain.
// Adding middleware invalidates any previously cached Build result.
// Must not be called concurrently with Execute.
func (m *MiddlewareChain) Use(middleware Middleware) *MiddlewareChain {
	m.mu.Lock()
	m.middlewares = append(m.middlewares, middleware)
	m.built = nil // invalidate cache
	m.mu.Unlock()
	return m
}

// Build pre-computes the composed handler chain and caches it.
// Subsequent calls to Execute will use the cached handler for better performance.
// Must not be called concurrently with Use.
// Returns the chain for method chaining.
func (m *MiddlewareChain) Build() *MiddlewareChain {
	m.mu.Lock()
	defer m.mu.Unlock()
	handler := m.handler
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		handler = m.middlewares[i](handler)
	}
	m.built = handler
	return m
}

// Execute runs the middleware chain and final handler.
// Automatically builds and caches the chain on first call if Build was not
// called explicitly. Safe for concurrent use once the chain is built.
// Uses double-checked locking to ensure only one goroutine builds.
func (m *MiddlewareChain) Execute(client *Client, msg *Message) error {
	m.mu.RLock()
	built := m.built
	m.mu.RUnlock()

	if built != nil {
		return built(client, msg)
	}

	// Upgrade to write lock and double-check before building.
	m.mu.Lock()
	if m.built == nil {
		handler := m.handler
		for i := len(m.middlewares) - 1; i >= 0; i-- {
			handler = m.middlewares[i](handler)
		}
		m.built = handler
	}
	built = m.built
	m.mu.Unlock()

	return built(client, msg)
}

// Built-in middleware examples

// LoggingMiddleware logs incoming messages.
func LoggingMiddleware(logger Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(client *Client, msg *Message) error {
			logger.Debug("Message received",
				"clientID", client.ID,
				"userID", client.GetUserID(),
				"size", len(msg.Data),
			)
			err := next(client, msg)
			if err != nil {
				logger.Error("Message handling failed",
					"clientID", client.ID,
					"error", err,
				)
			}
			return err
		}
	}
}

// RecoveryMiddleware recovers from panics in message handlers.
func RecoveryMiddleware(logger Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(client *Client, msg *Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic recovered in message handler",
						"clientID", client.ID,
						"panic", r,
					)
					err = ErrInvalidMessage
				}
			}()
			return next(client, msg)
		}
	}
}

// MetricsMiddleware records handler-level metrics for message processing.
// Note: message count and size are already tracked by the readPump, so this
// middleware only records handler errors and processing latency to avoid
// double-counting those. However, processing latency is also recorded
// internally by the hub when a message handler is set via WithMessageHandler.
// If you use MetricsMiddleware inside a chain passed to WithMessageHandler,
// add WithoutHandlerLatency() to your hub options to disable the hub's
// built-in latency recording and avoid double-counting.
func MetricsMiddleware(metrics MetricsCollector) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(client *Client, msg *Message) error {
			start := time.Now()
			err := next(client, msg)
			metrics.RecordLatency(time.Since(start))

			if err != nil {
				metrics.IncrementErrors("message_handling")
			}

			return err
		}
	}
}
