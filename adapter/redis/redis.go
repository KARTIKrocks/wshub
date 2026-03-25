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
	"sync"

	"github.com/KARTIKrocks/wshub"
	goredis "github.com/redis/go-redis/v9"
)

const defaultChannel = "wshub:messages"

// ErrClosed is returned when Publish is called after Close.
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

// Adapter implements wshub.Adapter using Redis Pub/Sub.
// It is safe for concurrent use.
type Adapter struct {
	client  goredis.UniversalClient
	channel string

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
		return err
	}
	return a.client.Publish(ctx, a.channel, data).Err()
}

// Subscribe begins receiving messages from Redis Pub/Sub. The handler is
// called for every message received. Subscribe spawns a goroutine internally
// and returns immediately.
//
// The subscription is stopped when the context is cancelled or Close is called.
func (a *Adapter) Subscribe(ctx context.Context, handler func(wshub.AdapterMessage)) error {
	ctx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()

	sub := a.client.Subscribe(ctx, a.channel)

	// Wait for confirmation that the subscription is active.
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		cancel()
		return err
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer sub.Close()

		ch := sub.Channel()
		for msg := range ch {
			var am wshub.AdapterMessage
			if err := json.Unmarshal([]byte(msg.Payload), &am); err != nil {
				continue // skip malformed — caller has no logger; rely on metrics
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
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	return nil
}
