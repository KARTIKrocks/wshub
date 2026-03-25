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

func setupClientTest(t *testing.T) (*Hub, func() *websocket.Conn) {
	t.Helper()
	hub := NewHub()
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
	dial := func() *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		conn, _, err := dialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	return hub, dial
}

func TestClientSetUserID(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	err := client.SetUserID("user-1")
	if err != nil {
		t.Fatalf("SetUserID: %v", err)
	}
	if client.GetUserID() != "user-1" {
		t.Errorf("GetUserID = %q, want user-1", client.GetUserID())
	}

	// Set same ID should be no-op
	err = client.SetUserID("user-1")
	if err != nil {
		t.Fatalf("SetUserID same: %v", err)
	}
}

func TestClientSetUserIDConcurrent(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			client.SetUserID("user-1")
		}(i)
	}
	wg.Wait()

	if client.GetUserID() != "user-1" {
		t.Errorf("GetUserID = %q, want user-1", client.GetUserID())
	}
}

func TestClientSetUserIDLimit(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxConnectionsPerUser(1)),
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

	conn1, _, _ := dialer.Dial(url, nil)
	defer conn1.Close()
	conn2, _, _ := dialer.Dial(url, nil)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	clients := hub.Clients()
	if len(clients) < 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	// First client sets user ID
	err := clients[0].SetUserID("user-1")
	if err != nil {
		t.Fatalf("SetUserID: %v", err)
	}

	// Second client should fail
	err = clients[1].SetUserID("user-1")
	if err != ErrMaxUserConnectionsReached {
		t.Errorf("got %v, want ErrMaxUserConnectionsReached", err)
	}
}

func TestClientSend(t *testing.T) {
	hub, dial := setupClientTest(t)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	err := client.Send([]byte("hello"))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != "hello" {
		t.Errorf("got %q, want %q", msg, "hello")
	}
}

func TestClientSendText(t *testing.T) {
	hub, dial := setupClientTest(t)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	client.SendText("text-msg")

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, _ := conn.ReadMessage()
	if string(msg) != "text-msg" {
		t.Errorf("got %q, want %q", msg, "text-msg")
	}
}

func TestClientSendJSON(t *testing.T) {
	hub, dial := setupClientTest(t)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	client.SendJSON(map[string]string{"k": "v"})

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, _ := conn.ReadMessage()
	if !strings.Contains(string(msg), `"k"`) {
		t.Errorf("got %q, want JSON with key", msg)
	}
}

func TestClientSendBinary(t *testing.T) {
	hub, dial := setupClientTest(t)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	client.SendBinary([]byte{0x01, 0x02, 0x03})

	conn.SetReadDeadline(time.Now().Add(time.Second))
	msgType, msg, _ := conn.ReadMessage()
	if msgType != websocket.BinaryMessage {
		t.Errorf("message type = %d, want binary", msgType)
	}
	if len(msg) != 3 {
		t.Errorf("msg len = %d, want 3", len(msg))
	}
}

func TestClientSendWithContext(t *testing.T) {
	hub, dial := setupClientTest(t)
	conn := dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := client.SendWithContext(ctx, []byte("ctx-msg"))
	if err != nil {
		t.Fatalf("SendWithContext: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, _ := conn.ReadMessage()
	if string(msg) != "ctx-msg" {
		t.Errorf("got %q, want %q", msg, "ctx-msg")
	}
}

func TestClientSendWithContextCanceled(t *testing.T) {
	// Test SendWithContext with a closed client (simpler than trying to fill buffer)
	hub := NewHub()

	client := &Client{
		ID:       "test",
		hub:      hub,
		send:     make(chan sendItem), // unbuffered — will block
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := client.SendWithContext(ctx, []byte("should-fail"))
	if err != context.Canceled {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

func TestClientMetadata(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	client.SetMetadata("key", "value")
	v, ok := client.GetMetadata("key")
	if !ok || v != "value" {
		t.Errorf("GetMetadata = (%v, %v), want (value, true)", v, ok)
	}

	_, ok = client.GetMetadata("missing")
	if ok {
		t.Error("GetMetadata should return false for missing key")
	}

	client.DeleteMetadata("key")
	_, ok = client.GetMetadata("key")
	if ok {
		t.Error("DeleteMetadata should remove the key")
	}
}

func TestClientClose(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	if client.IsClosed() {
		t.Error("client should not be closed initially")
	}

	client.Close()

	if !client.IsClosed() {
		t.Error("client should be closed after Close()")
	}

	closedAt := client.ClosedAt()
	if closedAt.IsZero() {
		t.Error("ClosedAt should be set after Close()")
	}
}

func TestClientCloseWithCode(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	err := client.CloseWithCode(websocket.CloseNormalClosure, "bye")
	if err != nil {
		t.Fatalf("CloseWithCode: %v", err)
	}

	// Double close should be no-op
	err = client.CloseWithCode(websocket.CloseNormalClosure, "bye")
	if err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestClientRooms(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	hub.JoinRoom(client, "room1")
	hub.JoinRoom(client, "room2")

	rooms := client.Rooms()
	if len(rooms) != 2 {
		t.Errorf("Rooms count = %d, want 2", len(rooms))
	}
	if !client.InRoom("room1") {
		t.Error("client should be in room1")
	}
	if client.RoomCount() != 2 {
		t.Errorf("RoomCount = %d, want 2", client.RoomCount())
	}
}

func TestClientConnectedAt(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	if client.ConnectedAt().IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestClientRequest(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	if client.Request() == nil {
		t.Error("Request should not be nil")
	}
}

func TestClientCallbacks(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	client.OnMessage(func(c *Client, m *Message) {})
	client.OnClose(func(c *Client) {})
	client.OnError(func(c *Client, err error) {})

	// Verify callbacks are stored
	client.callbackMu.RLock()
	hasMessage := client.onMessage != nil
	hasClose := client.onClose != nil
	hasError := client.onError != nil
	client.callbackMu.RUnlock()

	if !hasMessage {
		t.Error("onMessage should be set")
	}
	if !hasClose {
		t.Error("onClose should be set")
	}
	if !hasError {
		t.Error("onError should be set")
	}
}

func TestClientSendAfterClose(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]
	client.Close()

	err := client.Send([]byte("should fail"))
	if err != ErrConnectionClosed {
		t.Errorf("Send after close = %v, want ErrConnectionClosed", err)
	}
}

func TestClientSendJSONError(t *testing.T) {
	hub, dial := setupClientTest(t)
	dial()
	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	// channels can't be marshaled to JSON
	err := client.SendJSON(make(chan int))
	if err == nil {
		t.Error("expected error marshaling channel")
	}
}

func TestClientSendMessageBufferFull(t *testing.T) {
	hub := NewHub()

	// Create a client with a tiny buffer
	client := &Client{
		ID:       "buf-full",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	// Fill the buffer
	_ = client.SendMessage(TextMessage, []byte("fill"))

	// Next send should fail with ErrWriteTimeout
	err := client.SendMessage(TextMessage, []byte("overflow"))
	if err != ErrWriteTimeout {
		t.Errorf("got %v, want ErrWriteTimeout", err)
	}
}

func TestClientSendWithContextClosed(t *testing.T) {
	hub := NewHub()

	client := &Client{
		ID:       "closed-ctx",
		hub:      hub,
		send:     make(chan sendItem, 1),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
		closed:   true,
	}

	ctx := context.Background()
	err := client.SendWithContext(ctx, []byte("should-fail"))
	if err != ErrConnectionClosed {
		t.Errorf("got %v, want ErrConnectionClosed", err)
	}
}

func TestClientReadPumpMessageHandler(t *testing.T) {
	var (
		mu       sync.Mutex
		received []*Message
	)

	hub := NewHub(
		WithMessageHandler(func(c *Client, m *Message) error {
			mu.Lock()
			received = append(received, m)
			mu.Unlock()
			return nil
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
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send message from client side
	conn.WriteMessage(websocket.TextMessage, []byte("hello-hub"))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if string(received[0].Data) != "hello-hub" {
		t.Errorf("got %q, want %q", received[0].Data, "hello-hub")
	}
	mu.Unlock()
}

func TestClientReadPumpWithHooks(t *testing.T) {
	var (
		mu           sync.Mutex
		beforeCalled bool
		afterCalled  bool
		modifiedData string
	)

	hub := NewHub(
		WithHooks(Hooks{
			BeforeMessage: func(c *Client, m *Message) (*Message, error) {
				mu.Lock()
				beforeCalled = true
				mu.Unlock()
				// Modify message
				return &Message{
					Type:     m.Type,
					Data:     []byte("modified"),
					ClientID: m.ClientID,
					Time:     m.Time,
				}, nil
			},
			AfterMessage: func(c *Client, m *Message, err error) {
				mu.Lock()
				afterCalled = true
				modifiedData = string(m.Data)
				mu.Unlock()
			},
		}),
		WithMessageHandler(func(c *Client, m *Message) error {
			return nil
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
	conn, _, _ := dialer.Dial(url, nil)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	conn.WriteMessage(websocket.TextMessage, []byte("original"))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !beforeCalled {
		t.Error("BeforeMessage hook not called")
	}
	if !afterCalled {
		t.Error("AfterMessage hook not called")
	}
	if modifiedData != "modified" {
		t.Errorf("AfterMessage got data %q, want %q", modifiedData, "modified")
	}
	mu.Unlock()
}

func TestClientReadPumpBeforeMessageReject(t *testing.T) {
	var (
		mu            sync.Mutex
		handlerCalled bool
	)

	hub := NewHub(
		WithHooks(Hooks{
			BeforeMessage: func(c *Client, m *Message) (*Message, error) {
				return nil, errors.New("rejected")
			},
		}),
		WithMessageHandler(func(c *Client, m *Message) error {
			mu.Lock()
			handlerCalled = true
			mu.Unlock()
			return nil
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
	conn, _, _ := dialer.Dial(url, nil)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	conn.WriteMessage(websocket.TextMessage, []byte("should-be-rejected"))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if handlerCalled {
		t.Error("message handler should not be called when BeforeMessage rejects")
	}
	mu.Unlock()
}

func TestClientOnMessageCallback(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, _ := hub.UpgradeConnection(w, r)
		var mu sync.Mutex
		var got string
		client.OnMessage(func(c *Client, m *Message) {
			mu.Lock()
			got = string(m.Data)
			mu.Unlock()
		})
		// Store for assertion
		client.SetMetadata("mu", &mu)
		client.SetMetadata("got", &got)
	}))
	t.Cleanup(server.Close)

	dialer := websocket.Dialer{}
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, _ := dialer.Dial(url, nil)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	conn.WriteMessage(websocket.TextMessage, []byte("callback-msg"))
	time.Sleep(100 * time.Millisecond)

	client := hub.Clients()[0]
	muVal, _ := client.GetMetadata("mu")
	gotVal, _ := client.GetMetadata("got")
	mu := muVal.(*sync.Mutex)
	got := gotVal.(*string)

	mu.Lock()
	if *got != "callback-msg" {
		t.Errorf("OnMessage got %q, want %q", *got, "callback-msg")
	}
	mu.Unlock()
}

func TestClientOnCloseCallback(t *testing.T) {
	var (
		mu     sync.Mutex
		called bool
	)

	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, _ := hub.UpgradeConnection(w, r)
		client.OnClose(func(c *Client) {
			mu.Lock()
			called = true
			mu.Unlock()
		})
	}))
	t.Cleanup(server.Close)

	dialer := websocket.Dialer{}
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, _ := dialer.Dial(url, nil)

	time.Sleep(50 * time.Millisecond)
	conn.Close()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !called {
		t.Error("OnClose callback not called")
	}
	mu.Unlock()
}

func TestClientRateLimit(t *testing.T) {
	hub := NewHub(
		WithLimits(DefaultLimits().WithMaxMessageRate(2)),
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
	conn, _, _ := dialer.Dial(url, nil)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	client := hub.Clients()[0]

	// First 2 should pass
	if !client.checkRateLimit() {
		t.Error("first message should pass rate limit")
	}
	if !client.checkRateLimit() {
		t.Error("second message should pass rate limit")
	}
	// Third should fail
	if client.checkRateLimit() {
		t.Error("third message should fail rate limit")
	}

	// After window passes, should reset
	time.Sleep(time.Second + 10*time.Millisecond)
	if !client.checkRateLimit() {
		t.Error("after window reset, should pass")
	}
}
