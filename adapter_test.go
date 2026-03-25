package wshub

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// In-memory adapter + bus for testing multi-node behaviour without Redis.
// ---------------------------------------------------------------------------

// memoryBus connects multiple memoryAdapters, simulating a shared message bus.
// When one adapter publishes, the bus delivers to all *other* adapters' handlers.
type memoryBus struct {
	mu       sync.Mutex
	adapters []*memoryAdapter
}

func newMemoryBus() *memoryBus {
	return &memoryBus{}
}

// newAdapter creates a memoryAdapter attached to this bus.
func (b *memoryBus) newAdapter() *memoryAdapter {
	a := &memoryAdapter{bus: b}
	b.mu.Lock()
	b.adapters = append(b.adapters, a)
	b.mu.Unlock()
	return a
}

// deliver sends msg to every adapter except the sender.
func (b *memoryBus) deliver(sender *memoryAdapter, msg AdapterMessage) {
	b.mu.Lock()
	targets := make([]*memoryAdapter, 0, len(b.adapters))
	for _, a := range b.adapters {
		if a != sender {
			targets = append(targets, a)
		}
	}
	b.mu.Unlock()

	for _, a := range targets {
		a.mu.Lock()
		handler := a.handler
		a.mu.Unlock()
		if handler != nil {
			handler(msg)
		}
	}
}

// memoryAdapter is an in-memory Adapter for testing.
type memoryAdapter struct {
	bus     *memoryBus
	mu      sync.Mutex
	handler func(AdapterMessage)
	closed  bool
}

func (a *memoryAdapter) Publish(_ context.Context, msg AdapterMessage) error {
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return ErrConnectionClosed
	}
	a.bus.deliver(a, msg)
	return nil
}

func (a *memoryAdapter) Subscribe(_ context.Context, handler func(AdapterMessage)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handler = handler
	return nil
}

func (a *memoryAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	a.handler = nil
	return nil
}

// ---------------------------------------------------------------------------
// failAdapter always returns an error on Publish — used to test error paths.
// ---------------------------------------------------------------------------

type failAdapter struct {
	memoryAdapter
}

func (a *failAdapter) Publish(_ context.Context, _ AdapterMessage) error {
	return ErrConnectionClosed
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupHubPair creates two hubs connected via a memoryBus and returns
// dial functions for each, plus the hubs themselves.
func setupHubPair(t *testing.T) (
	hubA, hubB *Hub,
	dialA, dialB func() *websocket.Conn,
) {
	t.Helper()

	bus := newMemoryBus()
	adapterA := bus.newAdapter()
	adapterB := bus.newAdapter()

	hubA = NewHub(WithAdapter(adapterA))
	hubB = NewHub(WithAdapter(adapterB))

	go hubA.Run()
	go hubB.Run()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		hubA.Shutdown(ctx)
		hubB.Shutdown(ctx)
	})

	dialA = makeDialer(t, hubA)
	dialB = makeDialer(t, hubB)

	return hubA, hubB, dialA, dialB
}

func makeDialer(t *testing.T, hub *Hub) func() *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeConnection(w, r)
	}))
	t.Cleanup(server.Close)

	dialer := websocket.Dialer{}
	return func() *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := dialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
}

// readWithTimeout reads one message from conn with a deadline.
func readWithTimeout(conn *websocket.Conn, timeout time.Duration) ([]byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	return data, err
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAdapterBroadcast(t *testing.T) {
	hubA, _, dialA, dialB := setupHubPair(t)

	connA := dialA()
	connB := dialB()
	time.Sleep(50 * time.Millisecond) // wait for registration

	// Broadcast from hub A.
	hubA.Broadcast([]byte("hello from A"))

	// Client on hub A should receive it (local delivery).
	data, err := readWithTimeout(connA, time.Second)
	if err != nil {
		t.Fatalf("connA read: %v", err)
	}
	if string(data) != "hello from A" {
		t.Errorf("connA got %q, want %q", data, "hello from A")
	}

	// Client on hub B should also receive it (adapter relay).
	data, err = readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "hello from A" {
		t.Errorf("connB got %q, want %q", data, "hello from A")
	}
}

func TestAdapterBroadcastBinary(t *testing.T) {
	hubA, _, dialA, dialB := setupHubPair(t)

	connA := dialA()
	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	hubA.BroadcastBinary([]byte{0x01, 0x02, 0x03})

	// Verify local delivery.
	data, err := readWithTimeout(connA, time.Second)
	if err != nil {
		t.Fatalf("connA read: %v", err)
	}
	if len(data) != 3 || data[0] != 0x01 {
		t.Errorf("connA got %v, want [1 2 3]", data)
	}

	// Verify cross-node delivery.
	data, err = readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if len(data) != 3 || data[0] != 0x01 {
		t.Errorf("connB got %v, want [1 2 3]", data)
	}
}

func TestAdapterBroadcastExcept(t *testing.T) {
	hubA, _, dialA, dialB := setupHubPair(t)

	connA := dialA()
	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	// Get the client on hub A to exclude it.
	clients := hubA.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client on hubA, got %d", len(clients))
	}

	hubA.BroadcastExcept([]byte("not for A"), clients[0])

	// connA should NOT receive the message (excluded).
	_, err := readWithTimeout(connA, 200*time.Millisecond)
	if err == nil {
		t.Error("connA should not have received the message")
	}

	// connB should receive it via adapter.
	data, err := readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "not for A" {
		t.Errorf("connB got %q, want %q", data, "not for A")
	}
}

func TestAdapterBroadcastToRoom(t *testing.T) {
	hubA, hubB, dialA, dialB := setupHubPair(t)

	connA := dialA()
	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	// Get clients and put them both in room "general".
	clientsA := hubA.Clients()
	clientsB := hubB.Clients()

	if err := hubA.JoinRoom(clientsA[0], "general"); err != nil {
		t.Fatalf("join room A: %v", err)
	}
	if err := hubB.JoinRoom(clientsB[0], "general"); err != nil {
		t.Fatalf("join room B: %v", err)
	}

	// Broadcast to room from hub A.
	if err := hubA.BroadcastToRoom("general", []byte("room msg")); err != nil {
		t.Fatalf("broadcast to room: %v", err)
	}

	// Both clients should receive it.
	data, err := readWithTimeout(connA, time.Second)
	if err != nil {
		t.Fatalf("connA read: %v", err)
	}
	if string(data) != "room msg" {
		t.Errorf("connA got %q, want %q", data, "room msg")
	}

	data, err = readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "room msg" {
		t.Errorf("connB got %q, want %q", data, "room msg")
	}
}

func TestAdapterBroadcastToRoomExcept(t *testing.T) {
	hubA, hubB, dialA, dialB := setupHubPair(t)

	connA := dialA()
	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	clientsA := hubA.Clients()
	clientsB := hubB.Clients()

	_ = hubA.JoinRoom(clientsA[0], "room1")
	_ = hubB.JoinRoom(clientsB[0], "room1")

	// Broadcast to room1 excluding the client on hub A.
	err := hubA.BroadcastToRoomExcept("room1", []byte("excluded"), clientsA[0])
	if err != nil {
		t.Fatalf("broadcast to room except: %v", err)
	}

	// connA should NOT receive (excluded).
	_, err = readWithTimeout(connA, 200*time.Millisecond)
	if err == nil {
		t.Error("connA should not have received the message")
	}

	// connB should receive via adapter.
	data, err := readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "excluded" {
		t.Errorf("connB got %q, want %q", data, "excluded")
	}
}

func TestAdapterSendToUser(t *testing.T) {
	_, hubB, _, dialB := setupHubPair(t)

	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	// Set user ID on the client connected to hub B.
	clientsB := hubB.Clients()
	if err := clientsB[0].SetUserID("user-42"); err != nil {
		t.Fatalf("set user ID: %v", err)
	}

	// SendToUser from hub B's own node (local path).
	hubB.SendToUser("user-42", []byte("hi user"))

	data, err := readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "hi user" {
		t.Errorf("connB got %q, want %q", data, "hi user")
	}
}

func TestAdapterSendToUserCrossNode(t *testing.T) {
	hubA, hubB, _, dialB := setupHubPair(t)

	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	clientsB := hubB.Clients()
	if err := clientsB[0].SetUserID("user-99"); err != nil {
		t.Fatalf("set user ID: %v", err)
	}

	// Send from hub A — user is only on hub B.
	hubA.SendToUser("user-99", []byte("cross node"))

	data, err := readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "cross node" {
		t.Errorf("connB got %q, want %q", data, "cross node")
	}
}

func TestAdapterSendToClient(t *testing.T) {
	hubA, hubB, _, dialB := setupHubPair(t)

	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	clientsB := hubB.Clients()
	clientID := clientsB[0].ID

	// Send from hub A to a client that exists only on hub B.
	err := hubA.SendToClient(clientID, []byte("direct"))
	if err != nil {
		t.Fatalf("SendToClient: %v", err)
	}

	data, err := readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "direct" {
		t.Errorf("connB got %q, want %q", data, "direct")
	}
}

func TestAdapterNodeDedup(t *testing.T) {
	// Verify that a hub does not process its own adapter messages.
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
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	// Simulate receiving a message with this hub's own node ID.
	hub.handleAdapterMessage(AdapterMessage{
		NodeID:  hub.nodeID,
		Type:    AdapterBroadcast,
		MsgType: websocket.TextMessage,
		Data:    []byte("self"),
	})

	// The client should NOT receive the message.
	_, err := readWithTimeout(conn, 200*time.Millisecond)
	if err == nil {
		t.Error("client should not have received its own node's adapter message")
	}
}

func TestAdapterNilNoRegression(t *testing.T) {
	// All broadcast methods must work when adapter is nil (single-node mode).
	hub := NewHub() // no adapter
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast([]byte("single node"))

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "single node" {
		t.Errorf("got %q, want %q", data, "single node")
	}
}

func TestAdapterPublishErrorLocalDeliveryStillWorks(t *testing.T) {
	// If the adapter fails to publish, local delivery must still succeed.
	adapter := &failAdapter{}
	metrics := NewDebugMetrics()
	hub := NewHub(WithAdapter(adapter), WithMetrics(metrics))
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast([]byte("should still arrive"))

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "should still arrive" {
		t.Errorf("got %q, want %q", data, "should still arrive")
	}

	// Verify the adapter error was counted.
	stats := metrics.Stats()
	if stats.Errors["adapter_publish"] == 0 {
		t.Error("expected adapter_publish error to be counted")
	}
}

func TestAdapterBroadcastToRoomCrossNodeNoLocalRoom(t *testing.T) {
	// Hub A broadcasts to a room that only exists on hub B.
	hubA, hubB, _, dialB := setupHubPair(t)

	connB := dialB()
	time.Sleep(50 * time.Millisecond)

	clientsB := hubB.Clients()
	_ = hubB.JoinRoom(clientsB[0], "remote-only")

	// Room "remote-only" does not exist on hubA — should not error with adapter.
	err := hubA.BroadcastToRoom("remote-only", []byte("to remote"))
	if err != nil {
		t.Fatalf("BroadcastToRoom: %v", err)
	}

	data, err := readWithTimeout(connB, time.Second)
	if err != nil {
		t.Fatalf("connB read: %v", err)
	}
	if string(data) != "to remote" {
		t.Errorf("connB got %q, want %q", data, "to remote")
	}
}

func TestNodeID(t *testing.T) {
	hubA := NewHub()
	hubB := NewHub()

	if hubA.NodeID() == "" {
		t.Error("NodeID should not be empty")
	}
	if hubA.NodeID() == hubB.NodeID() {
		t.Error("two hubs should have different NodeIDs")
	}
}

type errorCloseAdapter struct {
	memoryAdapter
}

func (a *errorCloseAdapter) Close() error {
	return errors.New("adapter close error")
}

func TestHubShutdownAdapterCloseError(t *testing.T) {
	adapter := &errorCloseAdapter{}
	hub := NewHub(WithAdapter(adapter))
	go hub.Run()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should log adapter close error but still complete shutdown.
	err := hub.Shutdown(ctx)
	if err != nil {
		t.Logf("shutdown error (may be timeout): %v", err)
	}
}

func TestBroadcastExceptWithType_Adapter(t *testing.T) {
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
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	// BroadcastBinaryExcept with adapter exercises the adapter publish path.
	hub.BroadcastBinaryExcept([]byte{0x01}, clients[0])
}

func TestHandleAdapterMessage_BroadcastExcept(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	// Send an AdapterBroadcastExcept message — exclude the client.
	hub.handleAdapterMessage(AdapterMessage{
		NodeID:          "remote-node",
		Type:            AdapterBroadcastExcept,
		MsgType:         websocket.TextMessage,
		Data:            []byte("except-adapter"),
		ExceptClientIDs: []string{client.ID},
	})

	_, err := readWithTimeout(conn, 200*time.Millisecond)
	if err == nil {
		t.Error("excluded client should not receive the message")
	}
}

func TestHandleAdapterMessage_RoomBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	_ = hub.JoinRoom(client, "adapter-room")

	hub.handleAdapterMessage(AdapterMessage{
		NodeID:  "remote-node",
		Type:    AdapterRoom,
		MsgType: websocket.TextMessage,
		Data:    []byte("room-adapter"),
		Room:    "adapter-room",
	})

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "room-adapter" {
		t.Errorf("got %q, want %q", data, "room-adapter")
	}
}

func TestHandleAdapterMessage_RoomBroadcastExcept(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	_ = hub.JoinRoom(client, "adapter-except-room")

	hub.handleAdapterMessage(AdapterMessage{
		NodeID:          "remote-node",
		Type:            AdapterRoomExcept,
		MsgType:         websocket.TextMessage,
		Data:            []byte("room-except"),
		Room:            "adapter-except-room",
		ExceptClientIDs: []string{client.ID},
	})

	_, err := readWithTimeout(conn, 200*time.Millisecond)
	if err == nil {
		t.Error("excluded client should not receive the room message")
	}
}

func TestHandleAdapterMessage_SendToUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	_ = client.SetUserID("adapter-user")

	hub.handleAdapterMessage(AdapterMessage{
		NodeID:  "remote-node",
		Type:    AdapterUser,
		MsgType: websocket.TextMessage,
		Data:    []byte("user-adapter"),
		UserID:  "adapter-user",
	})

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "user-adapter" {
		t.Errorf("got %q, want %q", data, "user-adapter")
	}
}

func TestHandleAdapterMessage_SendToClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := makeDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	hub.handleAdapterMessage(AdapterMessage{
		NodeID:   "remote-node",
		Type:     AdapterClient,
		MsgType:  websocket.TextMessage,
		Data:     []byte("client-adapter"),
		ClientID: client.ID,
	})

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "client-adapter" {
		t.Errorf("got %q, want %q", data, "client-adapter")
	}
}

func TestHandleAdapterMessage_SendToClientNotFound(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	// Should not panic for nonexistent client.
	hub.handleAdapterMessage(AdapterMessage{
		NodeID:   "remote-node",
		Type:     AdapterClient,
		MsgType:  websocket.TextMessage,
		Data:     []byte("nope"),
		ClientID: "nonexistent",
	})
}
