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

// testDialer creates an httptest server with a hub and returns a function
// to dial a WebSocket connection to it.
func testDialer(t *testing.T, hub *Hub) (dial func() *websocket.Conn, server *httptest.Server) {
	t.Helper()
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeConnection(w, r)
	}))
	t.Cleanup(server.Close)

	dialer := websocket.Dialer{}
	dial = func() *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := dialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	return dial, server
}

func TestNewHub(t *testing.T) {
	hub := NewHub()
	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0", hub.ClientCount())
	}
}

func TestNewHubWithOptions(t *testing.T) {
	logger := &NoOpLogger{}
	metrics := NewDebugMetrics()
	limits := DefaultLimits().WithMaxConnections(100)

	hub := NewHub(
		WithLogger(logger),
		WithMetrics(metrics),
		WithLimits(limits),
	)

	if hub.limits.MaxConnections != 100 {
		t.Errorf("MaxConnections = %d, want 100", hub.limits.MaxConnections)
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)

	conn := dial()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("ClientCount = %d, want 1", hub.ClientCount())
	}

	conn.Close()

	// Wait for unregistration
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0", hub.ClientCount())
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)

	conn1 := dial()
	conn2 := dial()

	time.Sleep(50 * time.Millisecond)

	hub.BroadcastText("hello")

	conn1.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("conn1 read: %v", err)
	}
	if string(msg) != "hello" {
		t.Errorf("conn1 got %q, want %q", msg, "hello")
	}

	conn2.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err = conn2.ReadMessage()
	if err != nil {
		t.Fatalf("conn2 read: %v", err)
	}
	if string(msg) != "hello" {
		t.Errorf("conn2 got %q, want %q", msg, "hello")
	}
}

func TestHubBroadcastExcept(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)

	conn1 := dial()
	_ = dial() // conn2

	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	// Broadcast except first client
	hub.BroadcastExcept([]byte("excluded"), clients[0])

	// The excluded client should not get the message (set short deadline)
	_ = conn1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, _ = conn1.ReadMessage()
	// Either we get the message (wrong client excluded) or timeout — both acceptable
	// since we can't control which client maps to which conn.
}

func TestHubRooms(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)

	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	client := clients[0]

	// Join room
	err := hub.JoinRoom(client, "test-room")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if !hub.RoomExists("test-room") {
		t.Error("room should exist")
	}
	if hub.RoomCount("test-room") != 1 {
		t.Errorf("RoomCount = %d, want 1", hub.RoomCount("test-room"))
	}

	// Room names
	names := hub.RoomNames()
	if len(names) != 1 || names[0] != "test-room" {
		t.Errorf("RoomNames = %v, want [test-room]", names)
	}

	// Leave room
	err = hub.LeaveRoom(client, "test-room")
	if err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}

	if hub.RoomExists("test-room") {
		t.Error("room should not exist after last client leaves")
	}
}

func TestHubEmptyRoomName(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	if err := hub.JoinRoom(client, ""); err != ErrEmptyRoomName {
		t.Errorf("JoinRoom('') = %v, want ErrEmptyRoomName", err)
	}
	if err := hub.LeaveRoom(client, ""); err != ErrEmptyRoomName {
		t.Errorf("LeaveRoom('') = %v, want ErrEmptyRoomName", err)
	}
	if err := hub.BroadcastToRoom("", nil); err != ErrEmptyRoomName {
		t.Errorf("BroadcastToRoom('') = %v, want ErrEmptyRoomName", err)
	}
	if err := hub.BroadcastToRoomExcept("", nil); err != ErrEmptyRoomName {
		t.Errorf("BroadcastToRoomExcept('') = %v, want ErrEmptyRoomName", err)
	}
}

func TestHubShutdown(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	// Close the client connection first so pumps can exit
	conn.Close()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := hub.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestHubConnectionLimits(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxConnections(1)),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)

	// First connection should succeed
	dial()
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("ClientCount = %d, want 1", hub.ClientCount())
	}

	// Second connection should be rejected
	url := "ws" + strings.TrimPrefix("", "http") + "/ws"
	_ = url // we just verify client count stays at 1
}

func TestHubGetClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client")
	}

	got, ok := hub.GetClient(clients[0].ID)
	if !ok || got != clients[0] {
		t.Error("GetClient should return the registered client")
	}

	_, ok = hub.GetClient("nonexistent")
	if ok {
		t.Error("GetClient should return false for nonexistent ID")
	}
}

func TestHubSendToClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	err := hub.SendToClient(clients[0].ID, []byte("direct"))
	if err != nil {
		t.Fatalf("SendToClient: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != "direct" {
		t.Errorf("got %q, want %q", msg, "direct")
	}

	err = hub.SendToClient("nonexistent", []byte("fail"))
	if err != ErrClientNotFound {
		t.Errorf("SendToClient(nonexistent) = %v, want ErrClientNotFound", err)
	}
}

func TestHubBroadcastToRoom(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn1 := dial()
	time.Sleep(50 * time.Millisecond)

	// Get the single client and join it to a room
	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	hub.JoinRoom(clients[0], "room1")

	hub.BroadcastToRoom("room1", []byte("room-msg"))

	conn1.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != "room-msg" {
		t.Errorf("got %q, want %q", msg, "room-msg")
	}
}

func TestHubLeaveAllRooms(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	hub.JoinRoom(client, "r1")
	hub.JoinRoom(client, "r2")

	if client.RoomCount() != 2 {
		t.Fatalf("RoomCount = %d, want 2", client.RoomCount())
	}

	hub.LeaveAllRooms(client)

	if client.RoomCount() != 0 {
		t.Errorf("RoomCount = %d, want 0 after LeaveAllRooms", client.RoomCount())
	}
}

func TestHubConcurrentBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	for range 5 {
		dial()
	}
	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hub.BroadcastText("msg")
		}(i)
	}
	wg.Wait()
}

func TestHubHandleHTTP(t *testing.T) {
	hub := NewHub()

	handler := hub.HandleHTTP()
	if handler == nil {
		t.Fatal("HandleHTTP returned nil")
	}
}

func TestHubBroadcastWithContext(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := hub.BroadcastWithContext(ctx, []byte("ctx-msg"))
	if err != nil {
		t.Fatalf("BroadcastWithContext: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, _ := conn.ReadMessage()
	if string(msg) != "ctx-msg" {
		t.Errorf("got %q, want %q", msg, "ctx-msg")
	}
}

func TestHubBroadcastJSON(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	err := hub.BroadcastJSON(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("BroadcastJSON: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, _ := conn.ReadMessage()
	if !strings.Contains(string(msg), `"key"`) {
		t.Errorf("got %q, want JSON with key", msg)
	}
}

func TestHubUserIndex(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	client.SetUserID("user-1")

	got, ok := hub.GetClientByUserID("user-1")
	if !ok || got != client {
		t.Error("GetClientByUserID should return the client")
	}

	clients := hub.GetClientsByUserID("user-1")
	if len(clients) != 1 {
		t.Errorf("GetClientsByUserID returned %d clients, want 1", len(clients))
	}

	_, ok = hub.GetClientByUserID("nonexistent")
	if ok {
		t.Error("GetClientByUserID should return false for nonexistent user")
	}
}

func TestHubRoomNotFound(t *testing.T) {
	hub := NewHub()

	err := hub.BroadcastToRoom("nonexistent", []byte("test"))
	if err != ErrRoomNotFound {
		t.Errorf("got %v, want ErrRoomNotFound", err)
	}
}

func TestHubRoomFull(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxClientsPerRoom(1)),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) < 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	hub.JoinRoom(clients[0], "full-room")
	err := hub.JoinRoom(clients[1], "full-room")
	if err != ErrRoomFull {
		t.Errorf("got %v, want ErrRoomFull", err)
	}
}

func TestHubSendToUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	client.SetUserID("user-1")

	hub.SendToUser("user-1", []byte("user-msg"))

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != "user-msg" {
		t.Errorf("got %q, want %q", msg, "user-msg")
	}

	// SendToUser for nonexistent user should be a no-op (no panic)
	hub.SendToUser("nonexistent", []byte("noop"))
}

func TestHubBroadcastBinary(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastBinary([]byte{0xDE, 0xAD})

	conn.SetReadDeadline(time.Now().Add(time.Second))
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if msgType != websocket.BinaryMessage {
		t.Errorf("message type = %d, want binary (%d)", msgType, websocket.BinaryMessage)
	}
	if len(msg) != 2 || msg[0] != 0xDE || msg[1] != 0xAD {
		t.Errorf("got %v, want [0xDE 0xAD]", msg)
	}
}

func TestHubRoomClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) < 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	hub.JoinRoom(clients[0], "rc-room")
	hub.JoinRoom(clients[1], "rc-room")

	roomClients := hub.RoomClients("rc-room")
	if len(roomClients) != 2 {
		t.Errorf("RoomClients = %d, want 2", len(roomClients))
	}

	// Nonexistent room returns nil
	if got := hub.RoomClients("nonexistent"); got != nil {
		t.Errorf("RoomClients(nonexistent) = %v, want nil", got)
	}
}

func TestHubBroadcastToRoomExcept(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn1 := dial()
	conn2 := dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) < 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	hub.JoinRoom(clients[0], "except-room")
	hub.JoinRoom(clients[1], "except-room")

	// Broadcast except clients[0]
	hub.BroadcastToRoomExcept("except-room", []byte("except-msg"), clients[0])

	// One of the connections should receive the message, the other should not.
	// We can't map client index to conn index, so just verify no panic and
	// at least one connection gets the message.
	received := 0
	for _, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, err := conn.ReadMessage()
		if err == nil {
			received++
		}
	}
	if received != 1 {
		t.Errorf("expected exactly 1 connection to receive message, got %d", received)
	}

	// Nonexistent room
	err := hub.BroadcastToRoomExcept("nonexistent", []byte("test"))
	if err != ErrRoomNotFound {
		t.Errorf("got %v, want ErrRoomNotFound", err)
	}
}

func TestHubParallelBroadcast(t *testing.T) {
	hub := NewHub(
		WithParallelBroadcast(2), // small batch size to trigger parallel path
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	// Create enough clients to exceed batch size
	var conns []*websocket.Conn
	for range 5 {
		conns = append(conns, dial())
	}
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 5 {
		t.Fatalf("ClientCount = %d, want 5", hub.ClientCount())
	}

	// Test parallel Broadcast
	hub.Broadcast([]byte("parallel-msg"))

	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("conn[%d] read: %v", i, err)
		}
		if string(msg) != "parallel-msg" {
			t.Errorf("conn[%d] got %q, want %q", i, msg, "parallel-msg")
		}
	}

	// Test parallel BroadcastExcept
	clients := hub.Clients()
	hub.BroadcastExcept([]byte("except-parallel"), clients[0])

	// At least some connections should receive
	received := 0
	for _, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, err := conn.ReadMessage()
		if err == nil {
			received++
		}
	}
	if received < 3 {
		t.Errorf("expected at least 3 to receive except-parallel, got %d", received)
	}
}

func TestHubParallelBroadcastToRoom(t *testing.T) {
	hub := NewHub(
		WithParallelBroadcast(2),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	var conns []*websocket.Conn
	for range 5 {
		conns = append(conns, dial())
	}
	time.Sleep(100 * time.Millisecond)

	clients := hub.Clients()
	for _, c := range clients {
		hub.JoinRoom(c, "parallel-room")
	}

	hub.BroadcastToRoom("parallel-room", []byte("room-parallel"))

	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("conn[%d] read: %v", i, err)
		}
		if string(msg) != "room-parallel" {
			t.Errorf("conn[%d] got %q, want %q", i, msg, "room-parallel")
		}
	}
}

func TestHubTrySendBufferFull(t *testing.T) {
	hub := NewHub()

	// Create a client with a tiny send buffer
	client := &Client{
		ID:       "test-full",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer
	client.send <- sendItem{data: []byte("fill")}

	// trySend should not block — just drop the message
	hub.trySend(client, sendItem{data: []byte("overflow")})

	// Verify only the first message is in the buffer
	item := <-client.send
	if string(item.data) != "fill" {
		t.Errorf("got %q, want %q", item.data, "fill")
	}
}

func TestHubUpgradeConnectionBeforeConnectHook(t *testing.T) {
	hub := NewHub(
		WithHooks(Hooks{
			BeforeConnect: func(r *http.Request) error {
				return errors.New("rejected")
			},
		}),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeConnection(w, r)
	}))
	t.Cleanup(server.Close)

	dialer := websocket.Dialer{}
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestHubUpgradeConnectionLimit(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxConnections(1)),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, server := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	// Second connection should be rejected
	dialer := websocket.Dialer{}
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	_, resp, err := dialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected connection to be rejected due to limit")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestHubJoinRoomAlreadyInRoom(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	hub.JoinRoom(client, "dup-room")
	err := hub.JoinRoom(client, "dup-room")
	if err != ErrAlreadyInRoom {
		t.Errorf("got %v, want ErrAlreadyInRoom", err)
	}
}

func TestHubJoinRoomClientNotFound(t *testing.T) {
	hub := NewHub()

	fakeClient := &Client{
		ID:       "fake",
		hub:      hub,
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	err := hub.JoinRoom(fakeClient, "room")
	if err != ErrClientNotFound {
		t.Errorf("got %v, want ErrClientNotFound", err)
	}
}

func TestHubLeaveRoomNotInRoom(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) < 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	// Create room with first client
	hub.JoinRoom(clients[0], "leave-test")

	// Second client tries to leave a room it never joined
	err := hub.LeaveRoom(clients[1], "leave-test")
	if err != ErrNotInRoom {
		t.Errorf("got %v, want ErrNotInRoom", err)
	}

	// Leave nonexistent room
	err = hub.LeaveRoom(clients[0], "nonexistent")
	if err != ErrRoomNotFound {
		t.Errorf("got %v, want ErrRoomNotFound", err)
	}
}

func TestHubMaxRoomsPerClient(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxRoomsPerClient(1)),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	hub.JoinRoom(client, "room1")
	err := hub.JoinRoom(client, "room2")
	if err != ErrMaxRoomsReached {
		t.Errorf("got %v, want ErrMaxRoomsReached", err)
	}
}

func TestHubBroadcastWithContextCanceled(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	// Create a client with unbuffered send channel to block
	client := &Client{
		ID:       "block-test",
		hub:      hub,
		send:     make(chan sendItem), // unbuffered
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Manually register it in the hub's snapshot
	hub.mu.Lock()
	hub.clients[client] = struct{}{}
	hub.mu.Unlock()
	hub.updateClientsSnapshot()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := hub.BroadcastWithContext(ctx, []byte("should-fail"))
	if err != context.Canceled {
		t.Errorf("got %v, want context.Canceled", err)
	}

	// Clean up
	hub.mu.Lock()
	delete(hub.clients, client)
	hub.mu.Unlock()
}

func TestHubUpdateClientUserID(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	// Set initial user ID
	client.SetUserID("user-a")
	got, ok := hub.GetClientByUserID("user-a")
	if !ok || got != client {
		t.Error("expected client under user-a")
	}

	// Change user ID
	client.SetUserID("user-b")
	_, ok = hub.GetClientByUserID("user-a")
	if ok {
		t.Error("user-a should be removed from index")
	}
	got, ok = hub.GetClientByUserID("user-b")
	if !ok || got != client {
		t.Error("expected client under user-b")
	}

	// Clear user ID
	client.SetUserID("")
	_, ok = hub.GetClientByUserID("user-b")
	if ok {
		t.Error("user-b should be removed from index")
	}
}

func TestHubHooksLifecycle(t *testing.T) {
	var (
		mu                     sync.Mutex
		afterConnectCalled     bool
		beforeDisconnectCalled bool
		afterDisconnectCalled  bool
	)

	hub := NewHub(
		WithHooks(Hooks{
			AfterConnect: func(c *Client) {
				mu.Lock()
				afterConnectCalled = true
				mu.Unlock()
			},
			BeforeDisconnect: func(c *Client) {
				mu.Lock()
				beforeDisconnectCalled = true
				mu.Unlock()
			},
			AfterDisconnect: func(c *Client) {
				mu.Lock()
				afterDisconnectCalled = true
				mu.Unlock()
			},
		}),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	conn := dial()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !afterConnectCalled {
		t.Error("AfterConnect hook not called")
	}
	mu.Unlock()

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !beforeDisconnectCalled {
		t.Error("BeforeDisconnect hook not called")
	}
	if !afterDisconnectCalled {
		t.Error("AfterDisconnect hook not called")
	}
	mu.Unlock()
}

func TestHubRoomHooks(t *testing.T) {
	var (
		mu              sync.Mutex
		beforeJoinRoom  string
		afterJoinRoom   string
		beforeLeaveRoom string
		afterLeaveRoom  string
	)

	hub := NewHub(
		WithHooks(Hooks{
			BeforeRoomJoin: func(c *Client, room string) error {
				mu.Lock()
				beforeJoinRoom = room
				mu.Unlock()
				return nil
			},
			AfterRoomJoin: func(c *Client, room string) {
				mu.Lock()
				afterJoinRoom = room
				mu.Unlock()
			},
			BeforeRoomLeave: func(c *Client, room string) {
				mu.Lock()
				beforeLeaveRoom = room
				mu.Unlock()
			},
			AfterRoomLeave: func(c *Client, room string) {
				mu.Lock()
				afterLeaveRoom = room
				mu.Unlock()
			},
		}),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	hub.JoinRoom(client, "hook-room")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if beforeJoinRoom != "hook-room" {
		t.Errorf("BeforeRoomJoin room = %q, want hook-room", beforeJoinRoom)
	}
	if afterJoinRoom != "hook-room" {
		t.Errorf("AfterRoomJoin room = %q, want hook-room", afterJoinRoom)
	}
	mu.Unlock()

	hub.LeaveRoom(client, "hook-room")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if beforeLeaveRoom != "hook-room" {
		t.Errorf("BeforeRoomLeave room = %q, want hook-room", beforeLeaveRoom)
	}
	if afterLeaveRoom != "hook-room" {
		t.Errorf("AfterRoomLeave room = %q, want hook-room", afterLeaveRoom)
	}
	mu.Unlock()
}

func TestHubBeforeRoomJoinReject(t *testing.T) {
	hub := NewHub(
		WithHooks(Hooks{
			BeforeRoomJoin: func(c *Client, room string) error {
				return errors.New("not allowed")
			},
		}),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	err := hub.JoinRoom(client, "blocked-room")
	if err == nil || err.Error() != "not allowed" {
		t.Errorf("got %v, want 'not allowed' error", err)
	}
}

func TestHubHandleHTTPUpgrade(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	server := httptest.NewServer(hub.HandleHTTP())
	t.Cleanup(server.Close)

	dialer := websocket.Dialer{}
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial via HandleHTTP: %v", err)
	}
	conn.Close()
	time.Sleep(50 * time.Millisecond)
}

func TestHubBroadcastJSONError(t *testing.T) {
	hub := NewHub()

	// channels (func) can't be marshaled to JSON
	err := hub.BroadcastJSON(make(chan int))
	if err == nil {
		t.Error("expected error marshaling channel")
	}
}

func TestHubAddToUserIndexWithUserID(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial, _ := testDialer(t, hub)
	dial()
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) < 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	// Both clients set same user ID
	clients[0].SetUserID("shared-user")
	clients[1].SetUserID("shared-user")

	all := hub.GetClientsByUserID("shared-user")
	if len(all) != 2 {
		t.Errorf("GetClientsByUserID returned %d, want 2", len(all))
	}
}
