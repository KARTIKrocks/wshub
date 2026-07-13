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

// TestCloseWithCodeDoesNotRaceWithSenders pins the fix for a data race between
// CloseWithCode and the broadcast path.
//
// CloseWithCode used to close(c.send) while producers were still sending on that
// same channel — Hub.trySendErr's fast path, the DropOldest evict/enqueue loop,
// and SendMessageWithContext all send without holding any lock the closer took.
// Closing a channel concurrently with a send on it is a data race, and the
// recover() guard around the send only hides the panic that follows; it does not
// make the program correct. Under DropOldest it reproduced within a few hundred
// messages.
//
// Run with -race: on the old code this fails with "WARNING: DATA RACE" between
// Client.CloseWithCode's closechan and the chansend in trySendErr.
func TestCloseWithCodeDoesNotRaceWithSenders(t *testing.T) {
	hub := NewHub(WithDropPolicy(DropOldest))
	go hub.Run()
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, err := hub.UpgradeConnection(w, r)
		if err != nil {
			return
		}
		_ = hub.JoinRoom(client, "room")
	}))
	t.Cleanup(srv.Close)

	const clients = 8
	conns := make([]*websocket.Conn, 0, clients)
	for i := 0; i < clients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		conns = append(conns, conn)
	}
	t.Cleanup(func() {
		for _, c := range conns {
			_ = c.Close()
		}
	})

	// The clients never read, so every send buffer fills and the drop path — the
	// one that races with the close — runs hot.
	deadline := time.Now().Add(2 * time.Second)
	for hub.RoomCount("room") != clients && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := hub.RoomCount("room"); got != clients {
		t.Fatalf("joined %d clients, want %d", got, clients)
	}

	payload := make([]byte, 4<<10)
	var wg sync.WaitGroup

	// Broadcast hard...
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = hub.BroadcastToRoom("room", payload)
		}
	}()

	// ...while closing the clients underneath the broadcaster.
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		for _, c := range hub.RoomClients("room") {
			_ = c.CloseWithCode(1013, "too slow")
		}
	}()

	wg.Wait()
}
