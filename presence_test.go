package wshub

import (
	"context"
	"testing"
	"time"
)

func TestGlobalClientCountSingleNode(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	dial()
	dial()
	time.Sleep(50 * time.Millisecond)

	// Without adapter/presence, GlobalClientCount == ClientCount.
	if got := hub.GlobalClientCount(); got != 2 {
		t.Errorf("GlobalClientCount = %d, want 2", got)
	}
	if got := hub.ClientCount(); got != hub.GlobalClientCount() {
		t.Errorf("ClientCount(%d) != GlobalClientCount(%d)", got, hub.GlobalClientCount())
	}
}

func TestGlobalClientCountMultiNode(t *testing.T) {
	bus := newMemoryBus()
	adapterA := bus.newAdapter()
	adapterB := bus.newAdapter()

	presenceInterval := 100 * time.Millisecond

	hubA := NewHub(WithAdapter(adapterA), WithPresence(presenceInterval))
	hubB := NewHub(WithAdapter(adapterB), WithPresence(presenceInterval))

	go hubA.Run()
	go hubB.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		hubA.Shutdown(ctx)
		hubB.Shutdown(ctx)
	})

	dialA := makeDialer(t, hubA)
	dialB := makeDialer(t, hubB)

	// Connect 2 clients to A, 3 to B.
	dialA()
	dialA()
	dialB()
	dialB()
	dialB()
	time.Sleep(50 * time.Millisecond)

	// Wait for at least one presence tick.
	time.Sleep(2 * presenceInterval)

	// Hub A should see 2 local + 3 remote = 5.
	if got := hubA.GlobalClientCount(); got != 5 {
		t.Errorf("hubA.GlobalClientCount = %d, want 5", got)
	}

	// Hub B should see 3 local + 2 remote = 5.
	if got := hubB.GlobalClientCount(); got != 5 {
		t.Errorf("hubB.GlobalClientCount = %d, want 5", got)
	}

	// Local counts unchanged.
	if got := hubA.ClientCount(); got != 2 {
		t.Errorf("hubA.ClientCount = %d, want 2", got)
	}
	if got := hubB.ClientCount(); got != 3 {
		t.Errorf("hubB.ClientCount = %d, want 3", got)
	}
}

func TestGlobalRoomCountMultiNode(t *testing.T) {
	bus := newMemoryBus()
	adapterA := bus.newAdapter()
	adapterB := bus.newAdapter()

	presenceInterval := 100 * time.Millisecond

	hubA := NewHub(WithAdapter(adapterA), WithPresence(presenceInterval))
	hubB := NewHub(WithAdapter(adapterB), WithPresence(presenceInterval))

	go hubA.Run()
	go hubB.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		hubA.Shutdown(ctx)
		hubB.Shutdown(ctx)
	})

	dialA := makeDialer(t, hubA)
	dialB := makeDialer(t, hubB)
	dialA()
	dialB()
	dialB()
	time.Sleep(50 * time.Millisecond)

	// Put all clients in room "lobby".
	for _, c := range hubA.Clients() {
		if err := hubA.JoinRoom(c, "lobby"); err != nil {
			t.Fatalf("join room A: %v", err)
		}
	}
	for _, c := range hubB.Clients() {
		if err := hubB.JoinRoom(c, "lobby"); err != nil {
			t.Fatalf("join room B: %v", err)
		}
	}

	time.Sleep(2 * presenceInterval)

	// 1 on A + 2 on B = 3.
	if got := hubA.GlobalRoomCount("lobby"); got != 3 {
		t.Errorf("hubA.GlobalRoomCount(lobby) = %d, want 3", got)
	}
	if got := hubB.GlobalRoomCount("lobby"); got != 3 {
		t.Errorf("hubB.GlobalRoomCount(lobby) = %d, want 3", got)
	}

	// Non-existent room returns 0.
	if got := hubA.GlobalRoomCount("nonexistent"); got != 0 {
		t.Errorf("hubA.GlobalRoomCount(nonexistent) = %d, want 0", got)
	}
}

func TestPresenceStaleNodeEviction(t *testing.T) {
	bus := newMemoryBus()
	adapterA := bus.newAdapter()
	adapterB := bus.newAdapter()
	adapterC := bus.newAdapter()

	presenceInterval := 100 * time.Millisecond

	hubA := NewHub(WithAdapter(adapterA), WithPresence(presenceInterval))
	hubB := NewHub(WithAdapter(adapterB), WithPresence(presenceInterval))
	hubC := NewHub(WithAdapter(adapterC), WithPresence(presenceInterval))

	go hubA.Run()
	go hubB.Run()
	go hubC.Run()

	dialA := makeDialer(t, hubA)
	dialB := makeDialer(t, hubB)
	dialC := makeDialer(t, hubC)

	dialA()
	dialB()
	dialC()
	time.Sleep(50 * time.Millisecond)

	// Wait for presence to propagate.
	time.Sleep(2 * presenceInterval)

	if got := hubA.GlobalClientCount(); got != 3 {
		t.Errorf("before shutdown: hubA.GlobalClientCount = %d, want 3", got)
	}

	// Shut down hub C.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	hubC.Shutdown(ctx)

	// Wait for TTL (3x interval) + one extra tick to ensure eviction runs.
	time.Sleep(4 * presenceInterval)

	// Hub A should now see only 2 (self + B). Hub C is evicted.
	if got := hubA.GlobalClientCount(); got != 2 {
		t.Errorf("after eviction: hubA.GlobalClientCount = %d, want 2", got)
	}

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
		defer cancel2()
		hubA.Shutdown(ctx2)
		hubB.Shutdown(ctx2)
	})
}

func TestGlobalCountWithoutPresence(t *testing.T) {
	// Adapter set but WithPresence not called — should return local count only.
	bus := newMemoryBus()
	adapter := bus.newAdapter()

	hub := NewHub(WithAdapter(adapter))
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	dial()
	dial()
	time.Sleep(50 * time.Millisecond)

	if got := hub.GlobalClientCount(); got != 2 {
		t.Errorf("GlobalClientCount = %d, want 2 (local only)", got)
	}
}

func TestPresenceNoAdapterNoop(t *testing.T) {
	// WithPresence set but no adapter — should not panic, returns local count.
	hub := NewHub(WithPresence(100 * time.Millisecond))
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	if got := hub.GlobalClientCount(); got != 1 {
		t.Errorf("GlobalClientCount = %d, want 1", got)
	}
}

func TestPresenceConcurrentAccess(t *testing.T) {
	bus := newMemoryBus()
	adapterA := bus.newAdapter()
	adapterB := bus.newAdapter()

	presenceInterval := 50 * time.Millisecond

	hubA := NewHub(WithAdapter(adapterA), WithPresence(presenceInterval))
	hubB := NewHub(WithAdapter(adapterB), WithPresence(presenceInterval))

	go hubA.Run()
	go hubB.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		hubA.Shutdown(ctx)
		hubB.Shutdown(ctx)
	})

	dialA := makeDialer(t, hubA)
	dialB := makeDialer(t, hubB)

	// Connect clients while concurrently reading global counts.
	// This exercises the race detector on presenceMu + clients map.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_ = hubA.GlobalClientCount()
			_ = hubA.GlobalRoomCount("lobby")
			_ = hubB.GlobalClientCount()
			_ = hubB.GlobalRoomCount("lobby")
			time.Sleep(5 * time.Millisecond)
		}
	}()

	for i := 0; i < 5; i++ {
		dialA()
		dialB()
		time.Sleep(10 * time.Millisecond)
	}

	<-done

	// Just verify no panics or races occurred. Exact counts aren't important
	// here — the race detector is the real test.
	if got := hubA.GlobalClientCount(); got < 5 {
		t.Errorf("hubA.GlobalClientCount = %d, want >= 5", got)
	}
}

func TestWithNodeID(t *testing.T) {
	hub := NewHub(WithNodeID("my-pod-123"))
	if hub.NodeID() != "my-pod-123" {
		t.Errorf("NodeID = %q, want %q", hub.NodeID(), "my-pod-123")
	}
}

func TestWithNodeIDEmpty(t *testing.T) {
	hub := NewHub(WithNodeID(""))
	if hub.NodeID() == "" {
		t.Error("empty WithNodeID should not override default UUID")
	}
}

func TestGlobalRoomCountSingleNode(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	_ = hub.JoinRoom(clients[0], "room1")

	if got := hub.GlobalRoomCount("room1"); got != 1 {
		t.Errorf("GlobalRoomCount = %d, want 1", got)
	}
	if got := hub.GlobalRoomCount("room1"); got != hub.RoomCount("room1") {
		t.Errorf("GlobalRoomCount(%d) != RoomCount(%d)", got, hub.RoomCount("room1"))
	}
}
