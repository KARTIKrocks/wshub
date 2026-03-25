package wshub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Helper: dial with upgrade options
// ---------------------------------------------------------------------------

func testDialerWithOpts(t *testing.T, hub *Hub, opts ...UpgradeOption) func() *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeConnection(w, r, opts...)
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

// ---------------------------------------------------------------------------
// Binary variant tests
// ---------------------------------------------------------------------------

func TestBroadcastBinaryExcept(t *testing.T) {
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

	clients := hub.Clients()
	client1 := clients[0]

	conn2 := dial()
	time.Sleep(50 * time.Millisecond)

	// Exclude client1.
	hub.BroadcastBinaryExcept([]byte{0xAA, 0xBB}, client1)

	// conn2 should receive binary.
	data, err := readWithTimeout(conn2, time.Second)
	if err != nil {
		t.Fatalf("conn2 read: %v", err)
	}
	if len(data) != 2 || data[0] != 0xAA {
		t.Errorf("conn2 got %v, want [0xAA 0xBB]", data)
	}

	// conn1 should NOT receive (excluded).
	_, err = readWithTimeout(conn1, 200*time.Millisecond)
	if err == nil {
		t.Error("conn1 should not have received the message")
	}
}

func TestSendBinaryToUser(t *testing.T) {
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
	_ = clients[0].SetUserID("binary-user")

	hub.SendBinaryToUser("binary-user", []byte{0x01, 0x02})

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) != 2 || data[0] != 0x01 {
		t.Errorf("got %v, want [1 2]", data)
	}
}

func TestSendBinaryToClient(t *testing.T) {
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
	err := hub.SendBinaryToClient(clients[0].ID, []byte{0xFF})
	if err != nil {
		t.Fatalf("SendBinaryToClient: %v", err)
	}

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) != 1 || data[0] != 0xFF {
		t.Errorf("got %v, want [0xFF]", data)
	}
}

func TestBroadcastBinaryToRoom(t *testing.T) {
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
	_ = hub.JoinRoom(clients[0], "bin-room")

	err := hub.BroadcastBinaryToRoom("bin-room", []byte{0xDE, 0xAD})
	if err != nil {
		t.Fatalf("BroadcastBinaryToRoom: %v", err)
	}

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) != 2 || data[0] != 0xDE {
		t.Errorf("got %v, want [0xDE 0xAD]", data)
	}
}

func TestBroadcastBinaryToRoomExcept(t *testing.T) {
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

	// Identify client1 before dialing the second connection.
	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	client1 := clients[0]

	conn2 := dial()
	time.Sleep(50 * time.Millisecond)

	// Find client2.
	allClients := hub.Clients()
	var client2 *Client
	for _, c := range allClients {
		if c.ID != client1.ID {
			client2 = c
			break
		}
	}
	if client2 == nil {
		t.Fatal("could not find second client")
	}

	_ = hub.JoinRoom(client1, "bin-except-room")
	_ = hub.JoinRoom(client2, "bin-except-room")

	// Exclude client1 (conn1).
	err := hub.BroadcastBinaryToRoomExcept("bin-except-room", []byte{0xCA, 0xFE}, client1)
	if err != nil {
		t.Fatalf("BroadcastBinaryToRoomExcept: %v", err)
	}

	// conn2 should receive it (not excluded).
	data, err := readWithTimeout(conn2, time.Second)
	if err != nil {
		t.Fatalf("conn2 read: %v", err)
	}
	if len(data) != 2 || data[0] != 0xCA {
		t.Errorf("conn2 got %v, want [0xCA 0xFE]", data)
	}

	// conn1 should NOT receive (excluded).
	_, err = readWithTimeout(conn1, 200*time.Millisecond)
	if err == nil {
		t.Error("conn1 should not have received the message")
	}
}

// ---------------------------------------------------------------------------
// Context variant tests
// ---------------------------------------------------------------------------

func TestSendToUserWithContext(t *testing.T) {
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
	_ = clients[0].SetUserID("ctx-user")

	ctx := context.Background()
	err := hub.SendToUserWithContext(ctx, "ctx-user", []byte("ctx-msg"))
	if err != nil {
		t.Fatalf("SendToUserWithContext: %v", err)
	}

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "ctx-msg" {
		t.Errorf("got %q, want %q", data, "ctx-msg")
	}
}

func TestSendToUserWithContextNoClients(t *testing.T) {
	// When user has no local clients, should return nil (no-op locally,
	// adapter still fires).
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	err := hub.SendToUserWithContext(context.Background(), "nobody", []byte("hello"))
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestSendToClientWithContext(t *testing.T) {
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

	ctx := context.Background()
	err := hub.SendToClientWithContext(ctx, clients[0].ID, []byte("direct-ctx"))
	if err != nil {
		t.Fatalf("SendToClientWithContext: %v", err)
	}

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "direct-ctx" {
		t.Errorf("got %q, want %q", data, "direct-ctx")
	}
}

func TestSendToClientWithContextNotFound(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	err := hub.SendToClientWithContext(context.Background(), "nonexistent", []byte("nope"))
	if err != ErrClientNotFound {
		t.Errorf("got %v, want ErrClientNotFound", err)
	}
}

func TestSendToClientWithContextAdapter(t *testing.T) {
	// When client is not found locally but adapter is present, should publish
	// and return nil.
	bus := newMemoryBus()
	adapter := bus.newAdapter()

	hub := NewHub(WithAdapter(adapter))
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	err := hub.SendToClientWithContext(context.Background(), "remote-client", []byte("via-adapter"))
	if err != nil {
		t.Errorf("got %v, want nil (adapter should handle)", err)
	}
}

func TestBroadcastToRoomWithContext(t *testing.T) {
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
	_ = hub.JoinRoom(clients[0], "ctx-room")

	ctx := context.Background()
	err := hub.BroadcastToRoomWithContext(ctx, "ctx-room", []byte("room-ctx"))
	if err != nil {
		t.Fatalf("BroadcastToRoomWithContext: %v", err)
	}

	data, err := readWithTimeout(conn, time.Second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "room-ctx" {
		t.Errorf("got %q, want %q", data, "room-ctx")
	}
}

func TestBroadcastToRoomWithContextEmptyName(t *testing.T) {
	hub := NewHub()

	err := hub.BroadcastToRoomWithContext(context.Background(), "", []byte("x"))
	if err != ErrEmptyRoomName {
		t.Errorf("got %v, want ErrEmptyRoomName", err)
	}
}

func TestBroadcastToRoomWithContextNotFound(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	err := hub.BroadcastToRoomWithContext(context.Background(), "nonexistent", []byte("x"))
	if err != ErrRoomNotFound {
		t.Errorf("got %v, want ErrRoomNotFound", err)
	}
}

func TestBroadcastToRoomWithContextAdapter(t *testing.T) {
	// With adapter, room not found locally should still succeed (may exist on
	// another node).
	bus := newMemoryBus()
	adapter := bus.newAdapter()

	hub := NewHub(WithAdapter(adapter))
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	err := hub.BroadcastToRoomWithContext(context.Background(), "remote-room", []byte("x"))
	if err != nil {
		t.Errorf("got %v, want nil (adapter should relay)", err)
	}
}

// ---------------------------------------------------------------------------
// Option tests
// ---------------------------------------------------------------------------

func TestWithoutHandlerLatency(t *testing.T) {
	hub := NewHub(WithoutHandlerLatency())
	if !hub.skipHandlerLatency {
		t.Error("skipHandlerLatency should be true")
	}
}

func TestWithHookTimeout(t *testing.T) {
	hub := NewHub(WithHookTimeout(10 * time.Second))
	if hub.hookTimeout != 10*time.Second {
		t.Errorf("hookTimeout = %v, want 10s", hub.hookTimeout)
	}
}

func TestWithHookTimeoutZero(t *testing.T) {
	hub := NewHub(WithHookTimeout(0))
	if hub.hookTimeout != 5*time.Second {
		t.Errorf("hookTimeout = %v, want 5s (default)", hub.hookTimeout)
	}
}

func TestWithPresenceZeroInterval(t *testing.T) {
	bus := newMemoryBus()
	adapter := bus.newAdapter()

	hub := NewHub(WithAdapter(adapter), WithPresence(0))
	if hub.presenceInterval != 5*time.Second {
		t.Errorf("presenceInterval = %v, want 5s (default)", hub.presenceInterval)
	}
	if hub.presenceTTL != 15*time.Second {
		t.Errorf("presenceTTL = %v, want 15s (3x default)", hub.presenceTTL)
	}
}

func TestWithUserIDUpgradeOption(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxConnectionsPerUser(2)),
	)
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	dial := testDialerWithOpts(t, hub, WithUserID("user-abc"))
	dial()
	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}

	if uid := clients[0].GetUserID(); uid != "user-abc" {
		t.Errorf("userID = %q, want %q", uid, "user-abc")
	}
}

// ---------------------------------------------------------------------------
// NoOpLogger coverage
// ---------------------------------------------------------------------------

func TestNoOpLoggerMethods(t *testing.T) {
	l := &NoOpLogger{}
	// Just call all methods to cover them. They should not panic.
	l.Debug("msg", "key", "value")
	l.Info("msg", "key", "value")
	l.Warn("msg", "key", "value")
	l.Error("msg", "key", "value")
}
