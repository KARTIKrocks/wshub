package wshub

// HandlerFunc is a function that handles WebSocket messages.
type HandlerFunc func(*Client, *Message) error

// Middleware wraps a HandlerFunc to add additional functionality.
type Middleware func(HandlerFunc) HandlerFunc

// MiddlewareChain manages a chain of middlewares.
type MiddlewareChain struct {
	middlewares []Middleware
	handler     HandlerFunc
}

// NewMiddlewareChain creates a new middleware chain with the final handler.
func NewMiddlewareChain(handler HandlerFunc) *MiddlewareChain {
	return &MiddlewareChain{
		handler:     handler,
		middlewares: make([]Middleware, 0),
	}
}

// Use adds a middleware to the chain.
func (m *MiddlewareChain) Use(middleware Middleware) *MiddlewareChain {
	m.middlewares = append(m.middlewares, middleware)
	return m
}

// Execute runs the middleware chain and final handler.
func (m *MiddlewareChain) Execute(client *Client, msg *Message) error {
	handler := m.handler

	// Build chain from last to first
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		handler = m.middlewares[i](handler)
	}

	return handler(client, msg)
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

// MetricsMiddleware records metrics for message processing.
func MetricsMiddleware(metrics MetricsCollector) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(client *Client, msg *Message) error {
			metrics.IncrementMessages()
			metrics.RecordMessageSize(len(msg.Data))

			err := next(client, msg)

			if err != nil {
				metrics.IncrementErrors("message_handling")
			}

			return err
		}
	}
}
