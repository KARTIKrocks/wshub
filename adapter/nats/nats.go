// Package nats provides a NATS adapter for wshub multi-node communication.
// NATS offers lower latency than Redis Pub/Sub, making it well-suited for
// real-time WebSocket workloads.
//
// Usage:
//
//	nc, _ := gonats.Connect("nats://localhost:4222")
//	adapter := nats.New(nc)
//	hub := wshub.NewHub(wshub.WithAdapter(adapter))
//	go hub.Run()
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/KARTIKrocks/wshub"
	gonats "github.com/nats-io/nats.go"
)

const defaultSubject = "wshub.messages"

// ErrClosed is returned when Publish is called after Close.
var ErrClosed = errors.New("nats adapter: closed")

// Option configures the NATS adapter.
type Option func(*Adapter)

// WithSubject sets the NATS subject to publish and subscribe on.
// Default: "wshub.messages".
func WithSubject(subject string) Option {
	return func(a *Adapter) {
		if subject != "" {
			a.subject = subject
		}
	}
}

// WithUnmarshalErrorHandler sets a callback to handle JSON unmarshal errors
// in the Subscribe callback. If not set, unmarshal errors are silently ignored.
func WithUnmarshalErrorHandler(handler func(data []byte, err error)) Option {
	return func(a *Adapter) {
		a.unmarshalErrorHandler = handler
	}
}

// Adapter implements wshub.Adapter using NATS core Pub/Sub.
// It is safe for concurrent use.
type Adapter struct {
	conn                  *gonats.Conn
	subject               string
	unmarshalErrorHandler func(data []byte, err error)

	mu     sync.Mutex
	sub    *gonats.Subscription
	closed bool

	// stop releases the context-watcher goroutine started by Subscribe when
	// the subscription is torn down by Close rather than by cancellation.
	// Closed and cleared under mu, so it is closed at most once.
	stop chan struct{}
}

// New creates a new NATS adapter. The provided connection must already be
// established. Options can override defaults like the subject name.
func New(conn *gonats.Conn, opts ...Option) *Adapter {
	a := &Adapter{
		conn:    conn,
		subject: defaultSubject,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Publish sends an AdapterMessage to all other subscribed nodes via NATS.
// It serializes the message as JSON.
func (a *Adapter) Publish(ctx context.Context, msg wshub.AdapterMessage) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return ErrClosed
	}
	a.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return a.conn.Publish(a.subject, data)
}

// Subscribe begins receiving messages from NATS. The handler is called for
// every message received. Subscribe returns immediately — message delivery
// is handled by the NATS client's internal goroutine pool.
//
// The subscription is stopped when the context is cancelled, Close is
// called, or the NATS connection is closed.
func (a *Adapter) Subscribe(ctx context.Context, handler func(wshub.AdapterMessage)) error {
	// Drain any existing subscription, and release its watcher, to prevent
	// a leak when Subscribe is called more than once.
	a.mu.Lock()
	if a.sub != nil {
		_ = a.sub.Drain()
		a.sub = nil
	}
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	a.mu.Unlock()

	sub, err := a.conn.Subscribe(a.subject, func(msg *gonats.Msg) {
		var am wshub.AdapterMessage
		if err := json.Unmarshal(msg.Data, &am); err != nil {
			if a.unmarshalErrorHandler != nil {
				a.unmarshalErrorHandler(msg.Data, err)
			}
			return
		}
		handler(am)
	})
	if err != nil {
		return err
	}

	stop := make(chan struct{})

	a.mu.Lock()
	a.sub = sub
	a.stop = stop
	a.mu.Unlock()

	// Watch for context cancellation and drain the subscription. Callers
	// commonly pass a context that is never cancelled and tear down via Close
	// instead, so this goroutine must also exit on stop — otherwise it lives
	// for the life of the process. Exactly one of the two paths drains: on
	// stop, Close owns the drain and reports its error.
	go func() {
		select {
		case <-ctx.Done():
			_ = sub.Drain()
		case <-stop:
		}
	}()

	return nil
}

// Close unsubscribes and releases resources. It does not close the
// underlying NATS connection — that remains the caller's responsibility.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}

	if a.sub != nil {
		return a.sub.Drain()
	}
	return nil
}
