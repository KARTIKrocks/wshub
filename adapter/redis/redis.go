// Package redis provides a Redis Pub/Sub adapter for wshub multi-node
// communication.
//
// Usage:
//
//	rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
//	adapter := redis.New(rdb)
//	hub := wshub.NewHub(wshub.WithAdapter(adapter))
//	go hub.Run()
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/KARTIKrocks/wshub"
	goredis "github.com/redis/go-redis/v9"
)

const defaultChannel = "wshub:messages"

// ErrClosed is returned when Publish or Subscribe is called after Close.
var ErrClosed = errors.New("redis adapter: closed")

// Option configures the Redis adapter.
type Option func(*Adapter)

// WithChannel sets the Redis Pub/Sub channel name.
// Default: "wshub:messages".
func WithChannel(channel string) Option {
	return func(a *Adapter) {
		if channel != "" {
			a.channel = channel
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

// Adapter implements wshub.Adapter using Redis Pub/Sub.
// It is safe for concurrent use.
type Adapter struct {
	client                goredis.UniversalClient
	channel               string
	unmarshalErrorHandler func(data []byte, err error)

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

// New creates a new Redis adapter. The provided client must be connected
// and ready. Options can override defaults like the channel name.
func New(client goredis.UniversalClient, opts ...Option) *Adapter {
	a := &Adapter{
		client:  client,
		channel: defaultChannel,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Publish sends an AdapterMessage to all other subscribed nodes via Redis
// Pub/Sub. It serializes the message as JSON.
func (a *Adapter) Publish(ctx context.Context, msg wshub.AdapterMessage) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return ErrClosed
	}
	a.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if err := a.client.Publish(ctx, a.channel, data).Err(); err != nil {
		return fmt.Errorf("publish to channel %s: %w", a.channel, err)
	}
	return nil
}

// Subscribe begins receiving messages from Redis Pub/Sub. The handler is
// called for every message received. Subscribe spawns a goroutine internally
// and returns immediately.
//
// The subscription is stopped when the context is cancelled or Close is
// called. Calling Subscribe again replaces the previous subscription, which is
// released first. Subscribe returns ErrClosed if the adapter is closed.
func (a *Adapter) Subscribe(ctx context.Context, handler func(wshub.AdapterMessage)) error {
	ctx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		cancel()
		return ErrClosed
	}
	// Release any previous subscription before replacing it. Each
	// subscription's goroutines wait on the context captured when it was
	// created, so overwriting a.cancel without calling it strands both of
	// them and leaves Close blocked on wg.Wait forever.
	prev := a.cancel
	a.cancel = cancel
	a.mu.Unlock()

	if prev != nil {
		prev()
	}

	sub := a.client.Subscribe(ctx, a.channel)

	// Wait for confirmation that the subscription is active.
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		cancel()
		return err
	}

	// Closing the PubSub is what closes the channel the receive loop ranges
	// over, so it has to happen from outside that loop — a `defer sub.Close()`
	// on the receive goroutine can only run once the loop has already exited,
	// which never happens. go-redis does not tie the PubSub's lifetime to the
	// context passed to Subscribe, so this watcher is also what makes context
	// cancellation stop delivery.
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		<-ctx.Done()
		_ = sub.Close()
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		ch := sub.Channel()
		for msg := range ch {
			var am wshub.AdapterMessage
			if err := json.Unmarshal([]byte(msg.Payload), &am); err != nil {
				if a.unmarshalErrorHandler != nil {
					a.unmarshalErrorHandler([]byte(msg.Payload), err)
				}
				continue
			}
			handler(am)
		}
	}()

	return nil
}

// Close stops the subscriber goroutine and releases resources.
// It does not close the underlying Redis client — that remains the
// caller's responsibility.
func (a *Adapter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	cancel := a.cancel
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	a.wg.Wait()
	return nil
}
