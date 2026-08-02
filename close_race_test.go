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

	waitHubReady(t, hub)
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
	for range clients {
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

	waitFor(t, 2*time.Second, func() bool {
		return hub.RoomCount("room") == clients
	}, "all clients to join the room")

	payload := make([]byte, 4<<10)
	var wg sync.WaitGroup

	// Broadcast hard. The clients never read, so every send buffer fills and the
	// DropOldest evict/enqueue path — the one that races with the close — runs hot.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 2000 {
			_ = hub.BroadcastToRoom("room", payload)
		}
	}()

	// ...and close the clients underneath the broadcaster, but only once the send
	// buffers have actually backed up, so the close lands while senders are in
	// flight rather than before the broadcast has ramped up. Poll rather than
	// assert: this goroutine is not the test goroutine, so it must not call
	// t.Fatal, and a timeout here only makes the race window less likely to open,
	// never the test flaky.
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && !anyBufferFull(hub.RoomClients("room")) {
			time.Sleep(time.Millisecond)
		}
		for _, c := range hub.RoomClients("room") {
			_ = c.CloseWithCode(1013, "too slow")
		}
	}()

	wg.Wait()
}

// anyBufferFull reports whether any client's send buffer has backed up, i.e.
// the broadcaster has reached the drop path.
func anyBufferFull(clients []*Client) bool {
	for _, c := range clients {
		if len(c.send) == cap(c.send) {
			return true
		}
	}
	return false
}
