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

// Adapter implements wshub.Adapter using NATS core Pub/Sub.
// It is safe for concurrent use.
type Adapter struct {
	conn    *gonats.Conn
	subject string

	mu     sync.Mutex
	sub    *gonats.Subscription
	closed bool
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
	sub, err := a.conn.Subscribe(a.subject, func(msg *gonats.Msg) {
		var am wshub.AdapterMessage
		if err := json.Unmarshal(msg.Data, &am); err != nil {
			return // skip malformed — caller has no logger; rely on metrics
		}
		handler(am)
	})
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.sub = sub
	a.mu.Unlock()

	// Watch for context cancellation and drain the subscription.
	go func() {
		<-ctx.Done()
		_ = sub.Drain()
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

	if a.sub != nil {
		return a.sub.Drain()
	}
	return nil
}
