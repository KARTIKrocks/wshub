package wshub

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// ---------- helpers ----------

// mockClient creates a minimal client registered in the hub for benchmarking.
// The client has a buffered send channel so broadcasts don't block.
func mockClient(hub *Hub) *Client {
	c := &Client{
		ID:       fmt.Sprintf("client-%d", clientSeq.Add(1)),
		hub:      hub,
		send:     make(chan sendItem, 256),
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}
	hub.mu.Lock()
	hub.clients[c] = struct{}{}
	hub.clientIndex[c.ID] = c
	hub.mu.Unlock()
	return c
}

var clientSeq atomicInt64

type atomicInt64 struct{ v int64 }

func (a *atomicInt64) Add(n int64) int64 {
	a.v += n
	return a.v
}

// drainClient reads all messages from a client's send channel.
func drainClient(c *Client) {
	for {
		select {
		case <-c.send:
		default:
			return
		}
	}
}

// setupHubWithClients creates a hub with N mock clients and updates the snapshot.
func setupHubWithClients(n int) (*Hub, []*Client) {
	hub := NewHub()
	clients := make([]*Client, n)
	for i := range n {
		clients[i] = mockClient(hub)
	}
	hub.updateClientsSnapshot()
	return hub, clients
}

// setupHubWithRoom creates a hub with N clients all in the given room.
func setupHubWithRoom(n int, roomName string) (*Hub, []*Client) {
	hub, clients := setupHubWithClients(n)
	hub.roomsMu.Lock()
	room := &Room{clients: make(map[*Client]struct{}, n)}
	for _, c := range clients {
		room.clients[c] = struct{}{}
		c.rooms[roomName] = struct{}{}
	}
	hub.rooms[roomName] = room
	hub.roomsMu.Unlock()
	return hub, clients
}

// ---------- Message creation benchmarks ----------

func BenchmarkNewMessage(b *testing.B) {
	data := []byte("hello world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewMessage(data)
	}
}

func BenchmarkNewTextMessage(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewTextMessage("hello world")
	}
}

func BenchmarkNewBinaryMessage(b *testing.B) {
	data := make([]byte, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewBinaryMessage(data)
	}
}

func BenchmarkNewJSONMessage(b *testing.B) {
	payload := map[string]any{
		"type":    "chat",
		"message": "hello world",
		"from":    "user-123",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewJSONMessage(payload)
	}
}

func BenchmarkMessage_Text(b *testing.B) {
	msg := &Message{Data: []byte("hello world")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.Text()
	}
}

func BenchmarkMessage_JSON(b *testing.B) {
	data, _ := json.Marshal(map[string]string{"key": "value"})
	msg := &Message{Data: data}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]string
		_ = msg.JSON(&result)
	}
}

// ---------- Broadcast benchmarks ----------

func BenchmarkBroadcast(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000, 10000, 50000, 100000, 1000000}
	data := []byte("broadcast message payload")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithClients(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.Broadcast(data)
				for _, c := range clients {
					drainClient(c)
				}
			}
		})
	}
}

func BenchmarkBroadcastExcept(b *testing.B) {
	sizes := []int{5000, 100000, 1000000}
	data := []byte("broadcast except payload")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithClients(n)
			except := clients[0]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.BroadcastExcept(data, except)
				for _, c := range clients {
					drainClient(c)
				}
			}
		})
	}
}

func BenchmarkBroadcastParallel(b *testing.B) {
	sizes := []int{100, 1000, 5000, 10000, 50000}
	data := []byte("broadcast message payload")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithClients(n)
			hub.useParallel = true
			hub.parallelBatchSize = 100
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.Broadcast(data)
				for _, c := range clients {
					drainClient(c)
				}
			}
		})
	}
}

func BenchmarkBroadcastText(b *testing.B) {
	hub, clients := setupHubWithClients(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.BroadcastText("hello everyone")
		for _, c := range clients {
			drainClient(c)
		}
	}
}

func BenchmarkBroadcastJSON(b *testing.B) {
	hub, clients := setupHubWithClients(100)
	payload := map[string]string{"type": "update", "data": "value"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hub.BroadcastJSON(payload)
		for _, c := range clients {
			drainClient(c)
		}
	}
}

// ---------- Room broadcast benchmarks ----------

func BenchmarkBroadcastToRoom(b *testing.B) {
	sizes := []int{5000, 100000, 1000000}
	data := []byte("room message payload")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithRoom(n, "bench-room")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = hub.BroadcastToRoom("bench-room", data)
				for _, c := range clients {
					drainClient(c)
				}
			}
		})
	}
}

func BenchmarkBroadcastToRoomExcept(b *testing.B) {
	sizes := []int{5000, 100000, 1000000}
	data := []byte("room except payload")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithRoom(n, "bench-room")
			except := clients[0]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = hub.BroadcastToRoomExcept("bench-room", data, except)
				for _, c := range clients {
					drainClient(c)
				}
			}
		})
	}
}

// ---------- Client send benchmarks ----------

func BenchmarkClientSend(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]

	go func() {
		for range c.send {
		}
	}()

	data := []byte("test message")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Send(data)
	}
	hub.mu.Lock()
	delete(hub.clients, c)
	hub.mu.Unlock()
}

func BenchmarkClientSendJSON(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]

	go func() {
		for range c.send {
		}
	}()

	payload := map[string]string{"key": "value"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.SendJSON(payload)
	}
	hub.mu.Lock()
	delete(hub.clients, c)
	hub.mu.Unlock()
}

// ---------- Client lookup benchmarks ----------

func BenchmarkGetClient(b *testing.B) {
	hub, clients := setupHubWithClients(1000)
	targetID := clients[500].ID
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.GetClient(targetID)
	}
}

func BenchmarkClientCount(b *testing.B) {
	hub, _ := setupHubWithClients(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.ClientCount()
	}
}

func BenchmarkGetClientByUserID(b *testing.B) {
	hub, clients := setupHubWithClients(100)
	for i, c := range clients {
		c.mu.Lock()
		c.userID = fmt.Sprintf("user-%d", i)
		c.mu.Unlock()
		hub.userIndexMu.Lock()
		if hub.userIndex[c.userID] == nil {
			hub.userIndex[c.userID] = make(map[*Client]struct{})
		}
		hub.userIndex[c.userID][c] = struct{}{}
		hub.userIndexMu.Unlock()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.GetClientByUserID("user-50")
	}
}

// ---------- Room operation benchmarks ----------

func BenchmarkJoinRoom(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		roomName := fmt.Sprintf("room-%d", i)
		_ = hub.JoinRoom(c, roomName)
	}
}

func BenchmarkLeaveRoom(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]

	rooms := make([]string, b.N)
	for i := range b.N {
		rooms[i] = fmt.Sprintf("room-%d", i)
		_ = hub.JoinRoom(c, rooms[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hub.LeaveRoom(c, rooms[i])
	}
}

func BenchmarkRoomCount(b *testing.B) {
	hub, _ := setupHubWithRoom(100, "bench-room")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.RoomCount("bench-room")
	}
}

func BenchmarkRoomExists(b *testing.B) {
	hub, _ := setupHubWithRoom(10, "bench-room")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.RoomExists("bench-room")
	}
}

func BenchmarkRoomNames(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	for i := range 50 {
		_ = hub.JoinRoom(clients[0], fmt.Sprintf("room-%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.RoomNames()
	}
}

// ---------- Metadata benchmarks ----------

func BenchmarkSetMetadata(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]
	_ = hub
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.SetMetadata("key", "value")
	}
}

func BenchmarkGetMetadata(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]
	c.SetMetadata("key", "value")
	_ = hub
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.GetMetadata("key")
	}
}

// ---------- Middleware benchmarks ----------

func BenchmarkMiddlewareChain_Execute(b *testing.B) {
	noop := func(c *Client, m *Message) error { return nil }
	chain := NewMiddlewareChain(noop).
		Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Client, m *Message) error { return next(c, m) }
		}).
		Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Client, m *Message) error { return next(c, m) }
		}).
		Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Client, m *Message) error { return next(c, m) }
		}).
		Build()

	msg := &Message{Data: []byte("test")}
	client := &Client{ID: "bench", metadata: make(map[string]any), rooms: make(map[string]struct{})}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chain.Execute(client, msg)
	}
}

func BenchmarkMiddlewareChain_Unbuilt(b *testing.B) {
	noop := func(c *Client, m *Message) error { return nil }
	chain := NewMiddlewareChain(noop).
		Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Client, m *Message) error { return next(c, m) }
		}).
		Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Client, m *Message) error { return next(c, m) }
		}).
		Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Client, m *Message) error { return next(c, m) }
		})

	msg := &Message{Data: []byte("test")}
	client := &Client{ID: "bench", metadata: make(map[string]any), rooms: make(map[string]struct{})}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chain.Execute(client, msg)
	}
}

// ---------- Hub creation benchmarks ----------

func BenchmarkNewHub(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewHub()
	}
}

func BenchmarkNewHubWithOptions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewHub(
			WithConfig(DefaultConfig()),
			WithLogger(&NoOpLogger{}),
			WithMetrics(&NoOpMetrics{}),
			WithLimits(DefaultLimits()),
		)
	}
}

// ---------- Concurrent access benchmarks ----------

func BenchmarkConcurrentGetClient(b *testing.B) {
	hub, clients := setupHubWithClients(1000)
	targetID := clients[500].ID
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hub.GetClient(targetID)
		}
	})
}

func BenchmarkConcurrentClientCount(b *testing.B) {
	hub, _ := setupHubWithClients(1000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hub.ClientCount()
		}
	})
}

func BenchmarkConcurrentBroadcast(b *testing.B) {
	hub, clients := setupHubWithClients(100)
	data := []byte("concurrent broadcast")

	// Drain all clients in background
	var wg sync.WaitGroup
	done := make(chan struct{})
	for _, c := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case <-c.send:
				}
			}
		}(c)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hub.Broadcast(data)
		}
	})
	b.StopTimer()
	close(done)
	wg.Wait()
}

func BenchmarkConcurrentMetadata(b *testing.B) {
	hub, clients := setupHubWithClients(1)
	c := clients[0]
	_ = hub
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.SetMetadata("key", "value")
			c.GetMetadata("key")
		}
	})
}

// ---------- SendToUser / SendToClient benchmarks ----------

// setupHubWithUsers creates a hub with N clients, each assigned a unique user
// ID and registered in the user index.
func setupHubWithUsers(n int) (*Hub, []*Client) {
	hub, clients := setupHubWithClients(n)
	for i, c := range clients {
		c.mu.Lock()
		c.userID = fmt.Sprintf("user-%d", i)
		c.mu.Unlock()
		hub.userIndexMu.Lock()
		if hub.userIndex[c.userID] == nil {
			hub.userIndex[c.userID] = make(map[*Client]struct{})
		}
		hub.userIndex[c.userID][c] = struct{}{}
		hub.userIndexMu.Unlock()
	}
	return hub, clients
}

func BenchmarkSendToUser(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	data := []byte("hello user")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("users=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithUsers(n)
			target := clients[n/2]

			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-done:
						return
					case <-target.send:
					}
				}
			}()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.SendToUser(target.userID, data)
			}
			b.StopTimer()
			close(done)
		})
	}
}

func BenchmarkSendToClient(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	data := []byte("hello client")

	for _, n := range sizes {
		b.Run(fmt.Sprintf("clients=%d", n), func(b *testing.B) {
			hub, clients := setupHubWithClients(n)
			target := clients[n/2]

			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-done:
						return
					case <-target.send:
					}
				}
			}()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = hub.SendToClient(target.ID, data)
			}
			b.StopTimer()
			close(done)
		})
	}
}

// ---------- GlobalClientCount / GlobalRoomCount benchmarks ----------

func BenchmarkGlobalClientCount(b *testing.B) {
	nodeCounts := []int{5, 50, 100, 500}

	for _, nodes := range nodeCounts {
		b.Run(fmt.Sprintf("nodes=%d", nodes), func(b *testing.B) {
			hub, _ := setupHubWithClients(1000)
			hub.presenceCache = make(map[string]*nodeStats, nodes)
			for i := range nodes {
				hub.presenceCache[fmt.Sprintf("node-%d", i)] = &nodeStats{
					clientCount: 10000,
					rooms:       map[string]int{"lobby": 5000},
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.GlobalClientCount()
			}
		})
	}
}

func BenchmarkGlobalRoomCount(b *testing.B) {
	nodeCounts := []int{5, 50, 100, 500}

	for _, nodes := range nodeCounts {
		b.Run(fmt.Sprintf("nodes=%d", nodes), func(b *testing.B) {
			hub, _ := setupHubWithRoom(1000, "lobby")
			hub.presenceCache = make(map[string]*nodeStats, nodes)
			for i := range nodes {
				hub.presenceCache[fmt.Sprintf("node-%d", i)] = &nodeStats{
					clientCount: 10000,
					rooms:       map[string]int{"lobby": 5000},
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.GlobalRoomCount("lobby")
			}
		})
	}
}

// ---------- Message size benchmarks ----------

func BenchmarkBroadcast_MessageSize(b *testing.B) {
	sizes := []int{64, 512, 4096, 65536}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			hub, clients := setupHubWithClients(100)
			data := make([]byte, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hub.Broadcast(data)
				for _, c := range clients {
					drainClient(c)
				}
			}
		})
	}
}
