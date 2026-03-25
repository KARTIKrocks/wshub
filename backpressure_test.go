package wshub

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestDropNewestDefault(t *testing.T) {
	hub := NewHub() // default DropNewest

	client := &Client{
		ID:       "drop-newest",
		hub:      hub,
		send:     make(chan sendItem, 2),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer.
	client.send <- sendItem{data: []byte("msg1")}
	client.send <- sendItem{data: []byte("msg2")}

	// This should drop "msg3" (the new message).
	hub.trySend(client, sendItem{data: []byte("msg3")})

	// Buffer should still contain msg1, msg2.
	item1 := <-client.send
	item2 := <-client.send
	if string(item1.data) != "msg1" {
		t.Errorf("slot 0: got %q, want %q", item1.data, "msg1")
	}
	if string(item2.data) != "msg2" {
		t.Errorf("slot 1: got %q, want %q", item2.data, "msg2")
	}
}

func TestDropOldest(t *testing.T) {
	hub := NewHub(WithDropPolicy(DropOldest))

	client := &Client{
		ID:       "drop-oldest",
		hub:      hub,
		send:     make(chan sendItem, 2),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer.
	client.send <- sendItem{data: []byte("msg1")}
	client.send <- sendItem{data: []byte("msg2")}

	// This should evict "msg1" (oldest) and enqueue "msg3".
	hub.trySend(client, sendItem{data: []byte("msg3")})

	// Buffer should contain msg2, msg3.
	item1 := <-client.send
	item2 := <-client.send
	if string(item1.data) != "msg2" {
		t.Errorf("slot 0: got %q, want %q", item1.data, "msg2")
	}
	if string(item2.data) != "msg3" {
		t.Errorf("slot 1: got %q, want %q", item2.data, "msg3")
	}
}

func TestOnSendDroppedHookFired(t *testing.T) {
	var droppedCount atomic.Int32
	var lastDroppedData atomic.Value

	hub := NewHub(WithHooks(Hooks{
		OnSendDropped: func(c *Client, data []byte) {
			droppedCount.Add(1)
			lastDroppedData.Store(string(data))
		},
	}))

	client := &Client{
		ID:       "hook-test",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer.
	client.send <- sendItem{data: []byte("fill")}

	// This should trigger OnSendDropped with "overflow" (DropNewest).
	hub.trySend(client, sendItem{data: []byte("overflow")})

	if droppedCount.Load() != 1 {
		t.Errorf("dropped count = %d, want 1", droppedCount.Load())
	}
	if lastDroppedData.Load().(string) != "overflow" {
		t.Errorf("dropped data = %q, want %q", lastDroppedData.Load(), "overflow")
	}
}

func TestOnSendDroppedHookFiredForDropOldest(t *testing.T) {
	var droppedCount atomic.Int32
	var lastDroppedData atomic.Value

	hub := NewHub(
		WithDropPolicy(DropOldest),
		WithHooks(Hooks{
			OnSendDropped: func(c *Client, data []byte) {
				droppedCount.Add(1)
				lastDroppedData.Store(string(data))
			},
		}),
	)

	client := &Client{
		ID:       "hook-oldest",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer.
	client.send <- sendItem{data: []byte("old")}

	// DropOldest should evict "old" and enqueue "new".
	hub.trySend(client, sendItem{data: []byte("new")})

	if droppedCount.Load() != 1 {
		t.Errorf("dropped count = %d, want 1", droppedCount.Load())
	}
	// The dropped message should be "old" (the evicted one).
	if lastDroppedData.Load().(string) != "old" {
		t.Errorf("dropped data = %q, want %q", lastDroppedData.Load(), "old")
	}

	// The buffer should contain "new".
	item := <-client.send
	if string(item.data) != "new" {
		t.Errorf("buffer got %q, want %q", item.data, "new")
	}
}

func TestClientSendMessageDropOldest(t *testing.T) {
	hub := NewHub(WithDropPolicy(DropOldest))

	client := &Client{
		ID:       "client-oldest",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill via SendMessage.
	err := client.SendMessage(TextMessage, []byte("first"))
	if err != nil {
		t.Fatalf("first send: %v", err)
	}

	// Second send should succeed (evicts "first").
	err = client.SendMessage(TextMessage, []byte("second"))
	if err != nil {
		t.Fatalf("second send: %v", err)
	}

	item := <-client.send
	if string(item.data) != "second" {
		t.Errorf("got %q, want %q", item.data, "second")
	}
}

func TestOnSendDroppedNilHookNoopPanic(t *testing.T) {
	// Verify no panic when OnSendDropped is nil.
	hub := NewHub()

	client := &Client{
		ID:       "nil-hook",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	client.send <- sendItem{data: []byte("fill")}

	// Should not panic.
	hub.trySend(client, sendItem{data: []byte("overflow")})
}

func TestDropOldestMultipleOverflows(t *testing.T) {
	hub := NewHub(WithDropPolicy(DropOldest))

	client := &Client{
		ID:       "multi-overflow",
		hub:      hub,
		send:     make(chan sendItem, 2),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer.
	client.send <- sendItem{data: []byte("a")}
	client.send <- sendItem{data: []byte("b")}

	// Overflow three times.
	hub.trySend(client, sendItem{data: []byte("c")}) // evicts "a"
	hub.trySend(client, sendItem{data: []byte("d")}) // evicts "b"
	hub.trySend(client, sendItem{data: []byte("e")}) // evicts "c"

	// Buffer should contain "d", "e".
	item1 := <-client.send
	item2 := <-client.send
	if string(item1.data) != "d" {
		t.Errorf("slot 0: got %q, want %q", item1.data, "d")
	}
	if string(item2.data) != "e" {
		t.Errorf("slot 1: got %q, want %q", item2.data, "e")
	}
}

func TestWithDropPolicyOption(t *testing.T) {
	hub := NewHub(WithDropPolicy(DropOldest))
	if hub.dropPolicy != DropOldest {
		t.Errorf("dropPolicy = %d, want DropOldest(%d)", hub.dropPolicy, DropOldest)
	}
}

func TestDropOldestConcurrentSenders(t *testing.T) {
	// Multiple goroutines racing to send to the same client with DropOldest
	// and a tiny buffer. The race detector validates correctness.
	var dropCount atomic.Int64
	hub := NewHub(
		WithDropPolicy(DropOldest),
		WithHooks(Hooks{
			OnSendDropped: func(_ *Client, _ []byte) {
				dropCount.Add(1)
			},
		}),
	)

	client := &Client{
		ID:       "concurrent",
		hub:      hub,
		send:     make(chan sendItem, 2),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Drain the channel in a separate goroutine to simulate writePump.
	done := make(chan struct{})
	var received atomic.Int64
	go func() {
		defer close(done)
		for range client.send {
			received.Add(1)
		}
	}()

	// 10 goroutines each sending 100 messages.
	const goroutines = 10
	const perGoroutine = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				hub.trySend(client, sendItem{data: []byte("x")})
			}
		}()
	}
	wg.Wait()
	close(client.send)
	<-done

	total := received.Load() + dropCount.Load()
	if total != goroutines*perGoroutine {
		t.Errorf("received(%d) + dropped(%d) = %d, want %d",
			received.Load(), dropCount.Load(), total, goroutines*perGoroutine)
	}
}

func TestTrySendRecoverOnClosedChannel(t *testing.T) {
	// Verify that trySend does not panic when the channel is closed.
	hub := NewHub()

	client := &Client{
		ID:       "closed-chan",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	close(client.send)

	// Should not panic — the recover guard catches it.
	hub.trySend(client, sendItem{data: []byte("after close")})
}

func TestTrySendRecoverDropOldestOnClosedChannel(t *testing.T) {
	hub := NewHub(WithDropPolicy(DropOldest))

	client := &Client{
		ID:       "closed-oldest",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	close(client.send)

	// Should not panic with DropOldest either.
	hub.trySend(client, sendItem{data: []byte("after close")})
}

func TestSendMessageRecoverOnClosedChannel(t *testing.T) {
	hub := NewHub()

	client := &Client{
		ID:       "sendmsg-closed",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	close(client.send)

	// Should return error, not panic.
	err := client.SendMessage(TextMessage, []byte("test"))
	if err != ErrSendBufferFull {
		t.Errorf("got %v, want ErrSendBufferFull", err)
	}
}
