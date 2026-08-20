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
	"encoding/json/v2"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/KARTIKrocks/wshub"
	gonats "github.com/nats-io/nats.go"
)

const defaultSubject = "wshub.messages"

// ErrClosed is returned when Publish or Subscribe is called after Close.
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

// subscription bundles a NATS subscription with the signals used to tear it
// down exactly once. Draining is guarded by a sync.Once because two paths can
// reach it — the context watcher and Close — and NATS reports an error when
// the same subscription is drained twice.
type subscription struct {
	sub  *gonats.Subscription
	stop chan struct{} // closed to release the watcher without draining
	done chan struct{} // closed when the watcher has exited

	stopOnce  sync.Once
	drainOnce sync.Once
	drainErr  error
}

// release signals the watcher to exit and waits for it, so a caller that has
// taken ownership of this subscription knows no goroutine is still using it.
func (s *subscription) release() {
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done
}

// drain drains the underlying subscription at most once, returning the result
// of that single call to every caller.
func (s *subscription) drain() error {
	s.drainOnce.Do(func() { s.drainErr = s.sub.Drain() })
	return s.drainErr
}

// Adapter implements wshub.Adapter using NATS core Pub/Sub.
// It is safe for concurrent use.
type Adapter struct {
	conn                  *gonats.Conn
	subject               string
	unmarshalErrorHandler func(data []byte, err error)

	mu     sync.Mutex
	cur    *subscription
	closed bool

	// gen increments on every Subscribe. A call that finds the generation
	// changed while it was subscribing has been superseded and must not
	// install its subscription.
	gen uint64

	// watchers counts live watcher goroutines for this adapter. Only tests
	// read it; it is per-adapter so it stays exact under parallel tests.
	watchers atomic.Int64
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
// called, or the NATS connection is closed. Calling Subscribe again replaces
// the previous subscription, which is drained first. Subscribe returns
// ErrClosed if the adapter is closed.
func (a *Adapter) Subscribe(ctx context.Context, handler func(wshub.AdapterMessage)) error {
	// Take ownership of any existing subscription and claim a generation, so a
	// concurrent Subscribe or Close can be detected once the (unlocked)
	// conn.Subscribe below returns.
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return ErrClosed
	}
	a.gen++
	myGen := a.gen
	prev := a.cur
	a.cur = nil
	a.mu.Unlock()

	// Release the previous subscription outside the lock: release waits for
	// its watcher, which must not block on a mutex this call holds.
	if prev != nil {
		prev.release()
		_ = prev.drain()
	}

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

	s := &subscription{
		sub:  sub,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	// conn.Subscribe ran without the lock, so the adapter may have been closed
	// or superseded in the meantime. Installing unconditionally would leave a
	// live subscription on a closed adapter with a watcher nothing releases.
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		_ = s.drain()
		return ErrClosed
	}
	if a.gen != myGen {
		a.mu.Unlock()
		// A later Subscribe already replaced this one; it never becomes
		// visible, so drain it here rather than installing it.
		return s.drain()
	}
	a.cur = s
	a.watchers.Add(1)
	a.mu.Unlock()

	// Watch for context cancellation and drain the subscription. Callers
	// commonly pass a context that is never cancelled and tear down via Close
	// instead, so this goroutine must also exit on stop — otherwise it lives
	// for the life of the process. Draining is once-guarded, so it does not
	// matter which path gets there first.
	go func() {
		defer close(s.done)
		defer a.watchers.Add(-1)
		select {
		case <-ctx.Done():
			_ = s.drain()
		case <-s.stop:
		}
	}()

	return nil
}

// Close unsubscribes and releases resources. It does not close the
// underlying NATS connection — that remains the caller's responsibility.
// Close returns once the subscription's watcher goroutine has exited, so no
// goroutine started by Subscribe outlives it.
func (a *Adapter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	cur := a.cur
	a.cur = nil
	a.mu.Unlock()

	if cur == nil {
		return nil
	}
	// Outside the lock: release waits for the watcher, which must not block
	// on a mutex this call holds.
	cur.release()
	return cur.drain()
}
