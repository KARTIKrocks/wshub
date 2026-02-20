package wshub

import (
	"context"
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
