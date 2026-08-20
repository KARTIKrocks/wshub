package nats

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/KARTIKrocks/wshub"
	natsserver "github.com/nats-io/nats-server/v2/server"
	gonats "github.com/nats-io/nats.go"
)

// These exercise the actual publish/subscribe path against an embedded NATS
// server running in-process — no external broker is required, so they run in
// CI like any other test.

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

// newNATS starts an embedded NATS server and returns a connection to it.
func newNATS(t *testing.T) *gonats.Conn {
	t.Helper()

	srv, err := natsserver.NewServer(&natsserver.Options{
		Host: "127.0.0.1",
		Port: -1, // choose a free port
	})
	if err != nil {
		t.Fatalf("start embedded NATS server: %v", err)
	}
	go srv.Start()
	t.Cleanup(srv.Shutdown)

	if !srv.ReadyForConnections(recvTimeout) {
		t.Fatal("embedded NATS server not ready")
	}

	conn, err := gonats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("connect to embedded NATS: %v", err)
	}
	t.Cleanup(conn.Close)

	return conn
}

// subscribed starts a subscription and returns its collector, flushing the
// connection so the subscription is registered server-side before any publish.
func subscribed(t *testing.T, conn *gonats.Conn, a *Adapter) *collector {
	t.Helper()
	c := newCollector()
	if err := a.Subscribe(context.Background(), c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
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

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	c := subscribed(t, conn, a)

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

	conn := newNATS(t)

	publisher := New(conn)
	subA, subB := New(conn), New(conn)
	t.Cleanup(func() { _ = publisher.Close() })
	t.Cleanup(func() { _ = subA.Close() })
	t.Cleanup(func() { _ = subB.Close() })

	cA := subscribed(t, conn, subA)
	cB := subscribed(t, conn, subB)

	want := sampleMessage()
	if err := publisher.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	assertSameMessage(t, cA.next(t), want)
	assertSameMessage(t, cB.next(t), want)
}

// WithSubject must isolate traffic: a subscriber on a different subject must
// not observe the message.
func TestWithSubjectIsolatesTraffic(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)

	custom := New(conn, WithSubject("wshub.custom"))
	def := New(conn)
	t.Cleanup(func() { _ = custom.Close() })
	t.Cleanup(func() { _ = def.Close() })

	customC := subscribed(t, conn, custom)
	defaultC := subscribed(t, conn, def)

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

	conn := newNATS(t)

	type unmarshalErr struct {
		data []byte
		err  error
	}
	errCh := make(chan unmarshalErr, 8)

	a := New(conn, WithUnmarshalErrorHandler(func(data []byte, err error) {
		errCh <- unmarshalErr{data: data, err: err}
	}))
	t.Cleanup(func() { _ = a.Close() })

	c := subscribed(t, conn, a)

	if err := conn.Publish(defaultSubject, []byte("{not json")); err != nil {
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

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	c := subscribed(t, conn, a)

	if err := conn.Publish(defaultSubject, []byte("{not json")); err != nil {
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

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := a.Publish(context.Background(), sampleMessage()); !errors.Is(err, ErrClosed) {
		t.Errorf("Publish after Close = %v, want ErrClosed", err)
	}
}

func TestPublishHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := a.Publish(ctx, sampleMessage()); !errors.Is(err, context.Canceled) {
		t.Errorf("Publish with cancelled context = %v, want context.Canceled", err)
	}
}

// Close must stop delivery and return promptly.
func TestCloseStopsDelivery(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)

	sub := New(conn)
	t.Cleanup(func() { _ = sub.Close() })
	c := subscribed(t, conn, sub)

	publisher := New(conn)
	t.Cleanup(func() { _ = publisher.Close() })

	// Confirm delivery works before closing, so a later silence is
	// attributable to Close rather than a subscription that never started.
	if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c.next(t)

	done := make(chan error, 1)
	go func() { done <- sub.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(recvTimeout):
		t.Fatal("Close did not return")
	}

	// Drain is asynchronous; wait for delivery to actually stop.
	waitForSilence(t, c, publisher)
}

// Cancelling the Subscribe context must stop delivery, independently of Close.
func TestContextCancellationStopsDelivery(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)

	sub := New(conn)
	t.Cleanup(func() { _ = sub.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	c := newCollector()
	if err := sub.Subscribe(ctx, c.handle); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	publisher := New(conn)
	t.Cleanup(func() { _ = publisher.Close() })

	if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c.next(t)

	cancel()
	waitForSilence(t, c, publisher)
}

// waitForSilence polls until published messages stop being delivered, failing
// if delivery continues past recvTimeout.
func waitForSilence(t *testing.T, c *collector, publisher *Adapter) {
	t.Helper()

	deadline := time.Now().Add(recvTimeout)
	for time.Now().Before(deadline) {
		if err := publisher.Publish(context.Background(), sampleMessage()); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		select {
		case <-c.ch:
			// Still delivering — keep polling immediately.
		case <-time.After(quietPeriod):
			return // delivery stopped
		}
	}
	t.Fatal("delivery continued after the subscription was stopped")
}

// Subscribe drains any prior subscription rather than leaking it, so a second
// call must not produce duplicate deliveries.
func TestResubscribeDoesNotDuplicate(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	first := newCollector()
	if err := a.Subscribe(context.Background(), first.handle); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}

	second := subscribed(t, conn, a)

	want := sampleMessage()
	if err := a.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	assertSameMessage(t, second.next(t), want)

	// The drained first subscription may deliver in-flight messages, but must
	// not still be receiving newly published ones.
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

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := a.Subscribe(context.Background(), func(wshub.AdapterMessage) {})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Subscribe after Close = %v, want ErrClosed", err)
	}
}

// Repeated Subscribe must not strand goroutines that Close then waits on.
func TestCloseReturnsAfterRepeatedSubscribe(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

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
		t.Fatal("Close did not return after repeated Subscribe")
	}
}

// Close must release the goroutine Subscribe spawns to watch the context,
// even when that context is never cancelled by the caller. Close waits for
// that goroutine, so this asserts on the adapter's own watcher count rather
// than runtime.NumGoroutine, which is process-wide and would be perturbed by
// any other test running in parallel.
func TestCloseReleasesWatcherGoroutine(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })

	if err := a.Subscribe(context.Background(), func(wshub.AdapterMessage) {}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := a.watchers.Load(); got != 1 {
		t.Fatalf("watchers = %d after Subscribe, want 1", got)
	}

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := a.watchers.Load(); got != 0 {
		t.Errorf("watchers = %d after Close, want 0 — the context watcher leaked", got)
	}
}

// Close concurrent with Subscribe must not leave a live subscription on a
// closed adapter. conn.Subscribe runs without the adapter lock, so a naive
// implementation installs its result unconditionally and strands a watcher
// that nothing will ever release.
func TestConcurrentSubscribeAndClose(t *testing.T) {
	t.Parallel()

	for i := range 200 {
		conn := newNATS(t)
		a := New(conn)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = a.Subscribe(context.Background(), func(wshub.AdapterMessage) {})
		}()
		go func() {
			defer wg.Done()
			_ = a.Close()
		}()
		wg.Wait()

		a.mu.Lock()
		cur := a.cur
		closed := a.closed
		a.mu.Unlock()

		if closed && cur != nil {
			t.Fatalf("iteration %d: subscription installed on a closed adapter", i)
		}
		if got := a.watchers.Load(); got != 0 {
			t.Fatalf("iteration %d: watchers = %d after Close, want 0", i, got)
		}
	}
}

// Close is documented as idempotent and must stay safe under concurrent calls.
func TestCloseIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)
	a := New(conn)
	t.Cleanup(func() { _ = a.Close() })
	subscribed(t, conn, a)

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if err := a.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		})
	}
	wg.Wait()
}

// The adapter must tolerate concurrent publishers, since a hub fans out from
// many goroutines.
func TestConcurrentPublish(t *testing.T) {
	t.Parallel()

	conn := newNATS(t)

	sub := New(conn)
	t.Cleanup(func() { _ = sub.Close() })
	c := subscribed(t, conn, sub)

	publisher := New(conn)
	t.Cleanup(func() { _ = publisher.Close() })

	const publishers = 16
	var wg sync.WaitGroup
	for i := range publishers {
		wg.Go(func() {
			msg := sampleMessage()
			msg.ClientID = string(rune('a' + i))
			if err := publisher.Publish(context.Background(), msg); err != nil {
				t.Errorf("Publish: %v", err)
			}
		})
	}
	wg.Wait()

	for range publishers {
		c.next(t)
	}
}
