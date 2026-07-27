package redis

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/KARTIKrocks/wshub"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// These exercise the actual publish/subscribe path against miniredis, which
// runs in-process — no external Redis is required, so they run in CI like any
// other test.

const (
	// recvTimeout bounds how long a test waits for a message it expects.
	recvTimeout = 5 * time.Second
	// quietPeriod is how long a test waits to confirm a message it does not
	// expect never arrives.
	quietPeriod = 250 * time.Millisecond
)

// collector accumulates messages delivered to a Subscribe handler.
type collector struct {
	ch chan wshub.AdapterMessage
}

func newCollector() *collector {
	return &collector{ch: make(chan wshub.AdapterMessage, 64)}
}

func (c *collector) handle(msg wshub.AdapterMessage) {
	c.ch <- msg
}

// next returns the next delivered message, failing the test if none arrives.
func (c *collector) next(t *testing.T) wshub.AdapterMessage {
	t.Helper()
	select {
	case msg := <-c.ch:
		return msg
	case <-time.After(recvTimeout):
		t.Fatal("timed out waiting for a message")
		return wshub.AdapterMessage{}
	}
}

// expectQuiet fails the test if any message arrives within quietPeriod.
func (c *collector) expectQuiet(t *testing.T) {
	t.Helper()
	select {
	case msg := <-c.ch:
		t.Fatalf("unexpected message delivered: %+v", msg)
	case <-time.After(quietPeriod):
	}
}

// newRedis starts an in-process Redis and returns a connected client.
func newRedis(t *testing.T) *goredis.Client {
	t.Helper()
	srv := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// subscribed starts a subscription and returns its collector. Subscribe
// confirms the subscription is active before returning, so a Publish issued
// after this call is guaranteed to be delivered.
func subscribed(t *testing.T, a *Adapter) *collector {
	t.Helper()
	c := newCollector()
	if err := a.Subscribe(context.Background(), c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	return c
}

func sampleMessage() wshub.AdapterMessage {
	return wshub.AdapterMessage{
		NodeID:          "node-1",
		Type:            "broadcast_room",
		Room:            "general",
		UserID:          "user-7",
		ClientID:        "client-9",
		ExceptClientIDs: []string{"client-1", "client-2"},
		MsgType:         1,
		Data:            []byte("hello"),
	}
}

func assertSameMessage(t *testing.T, got, want wshub.AdapterMessage) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("message mismatch:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

// The core contract: what one node publishes, a subscribed node receives, with
// every field intact across the JSON round trip.
func TestPublishSubscribeRoundTrip(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)
	t.Cleanup(func() { _ = a.Close() })

	c := subscribed(t, a)

	want := sampleMessage()
	if err := a.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	assertSameMessage(t, c.next(t), want)
}

// Every subscriber receives every message — this is what makes hub fanout work
// across nodes. Deduplication of a node's own messages is the hub's job (via
// NodeID), not the adapter's.
func TestPublishReachesAllSubscribers(t *testing.T) {
	t.Parallel()

	client := newRedis(t)

	publisher := New(client)
	t.Cleanup(func() { _ = publisher.Close() })

	subA, subB := New(client), New(client)
	t.Cleanup(func() { _ = subA.Close() })
	t.Cleanup(func() { _ = subB.Close() })

	cA, cB := subscribed(t, subA), subscribed(t, subB)

	want := sampleMessage()
	if err := publisher.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	assertSameMessage(t, cA.next(t), want)
	assertSameMessage(t, cB.next(t), want)
}

// WithChannel must isolate traffic: a subscriber on a different channel must
// not observe the message.
func TestWithChannelIsolatesTraffic(t *testing.T) {
	t.Parallel()

	client := newRedis(t)

	custom := New(client, WithChannel("wshub:custom"))
	def := New(client)
	t.Cleanup(func() { _ = custom.Close() })
	t.Cleanup(func() { _ = def.Close() })

	customC, defaultC := subscribed(t, custom), subscribed(t, def)

	want := sampleMessage()
	if err := custom.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	assertSameMessage(t, customC.next(t), want)
	defaultC.expectQuiet(t)
}

// A malformed payload from another node must not kill the subscriber — the
// next valid message still has to arrive.
func TestMalformedPayloadDoesNotStopSubscription(t *testing.T) {
	t.Parallel()

	client := newRedis(t)

	type unmarshalErr struct {
		data []byte
		err  error
	}
	errCh := make(chan unmarshalErr, 8)

	a := New(client, WithUnmarshalErrorHandler(func(data []byte, err error) {
		errCh <- unmarshalErr{data: data, err: err}
	}))
	t.Cleanup(func() { _ = a.Close() })

	c := subscribed(t, a)

	if err := client.Publish(context.Background(), defaultChannel, "{not json").Err(); err != nil {
		t.Fatalf("publish malformed: %v", err)
	}

	select {
	case got := <-errCh:
		if got.err == nil {
			t.Error("unmarshal error handler called with nil error")
		}
		if string(got.data) != "{not json" {
			t.Errorf("handler data = %q, want %q", got.data, "{not json")
		}
	case <-time.After(recvTimeout):
		t.Fatal("timed out waiting for the unmarshal error handler")
	}

	// The subscription must still be live.
	want := sampleMessage()
	if err := a.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	assertSameMessage(t, c.next(t), want)
}

// Without a handler configured, a malformed payload is dropped silently and
// the subscription survives.
func TestMalformedPayloadWithoutHandler(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)
	t.Cleanup(func() { _ = a.Close() })

	c := subscribed(t, a)

	if err := client.Publish(context.Background(), defaultChannel, "{not json").Err(); err != nil {
		t.Fatalf("publish malformed: %v", err)
	}

	want := sampleMessage()
	if err := a.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	assertSameMessage(t, c.next(t), want)
}

func TestPublishAfterCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := a.Publish(context.Background(), sampleMessage()); !errors.Is(err, ErrClosed) {
		t.Errorf("Publish after Close = %v, want ErrClosed", err)
	}
}

// Close must return after Subscribe has been called. Regression test: the
// receive goroutine used to close the PubSub from its own defer, which could
// only run once the range loop exited — and that loop only exits when the
// PubSub is closed. Close blocked on wg.Wait() forever, hanging graceful
// shutdown on every multi-node deployment. Asserted with an explicit timeout
// so a reintroduced deadlock fails here instead of hanging the whole package.
func TestCloseReturnsAfterSubscribe(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)
	subscribed(t, a)

	done := make(chan error, 1)
	go func() { done <- a.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(recvTimeout):
		t.Fatal("Close did not return after Subscribe — subscriber goroutine deadlocked")
	}
}

// Regression test: Subscribe overwrote a.cancel without calling the previous
// one, so the first subscription's watcher and receive goroutines were
// stranded on a context nothing would ever cancel, and Close blocked on
// wg.Wait forever. The single-Subscribe deadlock was fixed before this case
// was; the NATS adapter already handled resubscribe, which is what exposed
// the asymmetry.
func TestCloseReturnsAfterRepeatedSubscribe(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)

	for i := range 3 {
		if err := a.Subscribe(context.Background(), func(wshub.AdapterMessage) {}); err != nil {
			t.Fatalf("Subscribe %d: %v", i, err)
		}
	}

	done := make(chan error, 1)
	go func() { done <- a.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(recvTimeout):
		t.Fatal("Close did not return after repeated Subscribe — goroutines stranded")
	}
}

// Resubscribing must replace the previous subscription, not run both.
func TestResubscribeReplacesPreviousSubscription(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)
	t.Cleanup(func() { _ = a.Close() })

	first := newCollector()
	if err := a.Subscribe(context.Background(), first.handle); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}

	second := subscribed(t, a)

	want := sampleMessage()
	if err := a.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	assertSameMessage(t, second.next(t), want)

	// The replaced subscription must not still be receiving.
	drainCollector(first)
	if err := a.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	second.next(t)
	first.expectQuiet(t)
}

func drainCollector(c *collector) {
	for {
		select {
		case <-c.ch:
		default:
			return
		}
	}
}

// Publish reports ErrClosed after Close, so Subscribe must too rather than
// silently resurrecting a closed adapter.
func TestSubscribeAfterCloseReturnsErrClosed(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := a.Subscribe(context.Background(), func(wshub.AdapterMessage) {})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Subscribe after Close = %v, want ErrClosed", err)
	}
}

// Close must stop delivery and release the subscriber goroutine.
func TestCloseStopsDelivery(t *testing.T) {
	t.Parallel()

	client := newRedis(t)

	sub := New(client)
	c := subscribed(t, sub)

	// Confirm delivery works before closing, so a later silence is
	// attributable to Close rather than a subscription that never started.
	publisher := New(client)
	t.Cleanup(func() { _ = publisher.Close() })

	if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c.next(t)

	if err := sub.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
		t.Fatalf("Publish after subscriber close: %v", err)
	}
	c.expectQuiet(t)
}

// Cancelling the Subscribe context must stop delivery, independently of Close.
func TestContextCancellationStopsDelivery(t *testing.T) {
	t.Parallel()

	client := newRedis(t)

	sub := New(client)
	t.Cleanup(func() { _ = sub.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	c := newCollector()
	if err := sub.Subscribe(ctx, c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	publisher := New(client)
	t.Cleanup(func() { _ = publisher.Close() })

	if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c.next(t)

	cancel()

	// Wait for the subscriber goroutine to observe the cancellation.
	deadline := time.Now().Add(recvTimeout)
	for time.Now().Before(deadline) {
		if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		select {
		case <-c.ch:
			time.Sleep(10 * time.Millisecond)
			continue
		case <-time.After(quietPeriod):
			return // delivery stopped
		}
	}
	t.Fatal("delivery continued after the subscription context was cancelled")
}

// Close is documented as idempotent and must stay safe under concurrent calls.
func TestCloseIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	client := newRedis(t)
	a := New(client)
	subscribed(t, a)

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
	}
	wg.Wait()
}

// The adapter must tolerate concurrent publishers, since a hub fans out from
// many goroutines.
func TestConcurrentPublish(t *testing.T) {
	t.Parallel()

	client := newRedis(t)

	sub := New(client)
	t.Cleanup(func() { _ = sub.Close() })
	c := subscribed(t, sub)

	publisher := New(client)
	t.Cleanup(func() { _ = publisher.Close() })

	const publishers = 16
	var wg sync.WaitGroup
	for i := range publishers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := sampleMessage()
			msg.ClientID = string(rune('a' + i))
			if err := publisher.Publish(context.Background(), msg); err != nil {
				t.Errorf("Publish: %v", err)
			}
		}()
	}
	wg.Wait()

	for range publishers {
		c.next(t)
	}
}
