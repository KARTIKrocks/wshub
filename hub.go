package websocket

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

// Room represents a chat room with its own lock for better concurrency.
type Room struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	// Clients is the set of registered clients.
	clients map[*Client]bool

	// Lock-free snapshot for broadcasting
	clientsSnapshot atomic.Value // map[*Client]bool

	// Rooms with per-room locks
	rooms   map[string]*Room
	roomsMu sync.RWMutex

	// User ID index for O(1) lookups
	userIndex   map[string]map[*Client]bool // userID -> clients
	userIndexMu sync.RWMutex

	// Channels for client management.
	register   chan *Client
	unregister chan *Client

	// Configuration
	config   Config
	limits   Limits
	upgrader websocket.Upgrader

	// Mutex for thread-safe operations on clients map
	mu sync.RWMutex

	// Message handler
	onMessage func(*Client, *Message) error

	// Hooks for lifecycle events
	hooks Hooks

	// Logger
	logger Logger

	// Metrics
	metrics MetricsCollector

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Parallel broadcast configuration
	parallelBatchSize int  // Number of clients per goroutine (default: 100)
	useParallel       bool // Enable parallel broadcasting (default: false, enable for 1000+ clients)
}

// NewHub creates a new WebSocket hub.
func NewHub(config Config) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	upgrader := websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		CheckOrigin:       config.CheckOrigin,
		EnableCompression: config.EnableCompression,
		Subprotocols:      config.Subprotocols,
	}

	h := &Hub{
		clients:           make(map[*Client]bool),
		rooms:             make(map[string]*Room),
		userIndex:         make(map[string]map[*Client]bool),
		register:          make(chan *Client),
		unregister:        make(chan *Client),
		config:            config,
		limits:            DefaultLimits(),
		upgrader:          upgrader,
		logger:            &NoOpLogger{},
		metrics:           &NoOpMetrics{},
		ctx:               ctx,
		cancel:            cancel,
		parallelBatchSize: 100,   // Default: 100 clients per goroutine
		useParallel:       false, // Default: disabled (enable for high client counts)
	}

	// Initialize snapshot with empty map
	h.clientsSnapshot.Store(make(map[*Client]bool))

	return h
}

// SetParallelBroadcast enables or disables parallel broadcasting.
// batchSize determines how many clients each goroutine handles (recommended: 50-200).
func (h *Hub) SetParallelBroadcast(enabled bool, batchSize int) {
	h.useParallel = enabled
	if batchSize > 0 {
		h.parallelBatchSize = batchSize
	}
}

// updateClientsSnapshot creates a new snapshot of clients for lock-free reads.
func (h *Hub) updateClientsSnapshot() {
	h.mu.RLock()
	snapshot := make(map[*Client]bool, len(h.clients))
	for client := range h.clients {
		snapshot[client] = true
	}
	h.mu.RUnlock()

	h.clientsSnapshot.Store(snapshot)
}

// addToUserIndex adds a client to the user index.
func (h *Hub) addToUserIndex(client *Client) {
	userID := client.GetUserID()
	if userID == "" {
		return
	}

	h.userIndexMu.Lock()
	if h.userIndex[userID] == nil {
		h.userIndex[userID] = make(map[*Client]bool)
	}
	h.userIndex[userID][client] = true
	h.userIndexMu.Unlock()
}

// removeFromUserIndex removes a client from the user index.
func (h *Hub) removeFromUserIndex(client *Client) {
	userID := client.GetUserID()
	if userID == "" {
		return
	}

	h.userIndexMu.Lock()
	if clients, ok := h.userIndex[userID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.userIndex, userID)
		}
	}
	h.userIndexMu.Unlock()
}

// UpdateClientUserID updates the user index when a client's user ID changes.
func (h *Hub) UpdateClientUserID(client *Client, oldUserID, newUserID string) {
	if oldUserID == newUserID {
		return
	}

	if oldUserID != "" {
		h.userIndexMu.Lock()
		if clients, ok := h.userIndex[oldUserID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.userIndex, oldUserID)
			}
		}
		h.userIndexMu.Unlock()
	}

	if newUserID != "" {
		h.userIndexMu.Lock()
		if h.userIndex[newUserID] == nil {
			h.userIndex[newUserID] = make(map[*Client]bool)
		}
		h.userIndex[newUserID][client] = true
		h.userIndexMu.Unlock()
	}
}

// SetLogger sets the logger for the hub.
func (h *Hub) SetLogger(logger Logger) {
	h.logger = logger
}

// SetMetrics sets the metrics collector for the hub.
func (h *Hub) SetMetrics(metrics MetricsCollector) {
	h.metrics = metrics
}

// SetLimits sets the limits for the hub.
func (h *Hub) SetLimits(limits Limits) {
	h.limits = limits
}

// SetHooks sets the lifecycle hooks for the hub.
func (h *Hub) SetHooks(hooks Hooks) {
	h.hooks = hooks
}

// OnMessage sets the message handler.
func (h *Hub) OnMessage(fn func(*Client, *Message) error) {
	h.onMessage = fn
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	h.wg.Add(1)
	defer h.wg.Done()

	for {
		select {
		case <-h.ctx.Done():
			// Shutdown: close all client connections
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
			}
			h.mu.Unlock()
			h.logger.Info("Hub shutdown complete")
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			// Update snapshot after adding client
			h.updateClientsSnapshot()

			// Add to user index if user ID is set
			h.addToUserIndex(client)

			h.metrics.IncrementConnections()
			h.logger.Info("Client registered",
				"clientID", client.ID,
				"totalClients", h.ClientCount(),
			)

			if h.hooks.AfterConnect != nil {
				go h.hooks.AfterConnect(client)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)

				// Remove from user index
				h.removeFromUserIndex(client)

				// Remove from all rooms
				h.removeClientFromAllRooms(client)
			}
			h.mu.Unlock()

			// Update snapshot after removing client
			h.updateClientsSnapshot()

			h.metrics.DecrementConnections()
			h.logger.Info("Client unregistered",
				"clientID", client.ID,
				"totalClients", h.ClientCount(),
			)

			// Call BeforeDisconnect hook
			if h.hooks.BeforeDisconnect != nil {
				h.hooks.BeforeDisconnect(client)
			}

			// Call client close handler
			client.callbackMu.RLock()
			closeHandler := client.onClose
			client.callbackMu.RUnlock()

			if closeHandler != nil {
				go closeHandler(client)
			}

			// Call AfterDisconnect hook
			if h.hooks.AfterDisconnect != nil {
				go h.hooks.AfterDisconnect(client)
			}
		}
	}
}

// removeClientFromAllRooms removes a client from all rooms (called with h.mu held).
func (h *Hub) removeClientFromAllRooms(client *Client) {
	for room := range client.rooms {
		h.roomsMu.Lock()
		if r, ok := h.rooms[room]; ok {
			r.mu.Lock()
			delete(r.clients, client)
			if len(r.clients) == 0 {
				delete(h.rooms, room)
			}
			r.mu.Unlock()
		}
		h.roomsMu.Unlock()
	}
}

// Shutdown gracefully shuts down the hub.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.logger.Info("Shutting down hub")
	h.cancel()

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		h.logger.Info("Hub shutdown successful")
		return nil
	case <-ctx.Done():
		h.logger.Warn("Hub shutdown timeout")
		return ctx.Err()
	}
}

// HandleHTTP returns an HTTP handler that upgrades connections to WebSocket.
func (h *Hub) HandleHTTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.HandleRequest(w, r)
	}
}

// HandleRequest upgrades an HTTP connection to WebSocket.
func (h *Hub) HandleRequest(w http.ResponseWriter, r *http.Request) (*Client, error) {
	// Call BeforeConnect hook
	if h.hooks.BeforeConnect != nil {
		if err := h.hooks.BeforeConnect(r); err != nil {
			h.logger.Warn("Connection rejected by BeforeConnect hook", "error", err)
			h.metrics.IncrementErrors("connection_rejected")
			http.Error(w, "Connection rejected", http.StatusForbidden)
			return nil, err
		}
	}

	// Check connection limits
	if err := h.canAcceptConnection(); err != nil {
		h.logger.Warn("Connection limit reached")
		h.metrics.IncrementErrors("connection_limit")
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return nil, err
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection", "error", err)
		h.metrics.IncrementErrors("upgrade_failed")
		return nil, err
	}

	client := newClient(h, conn, h.config)

	// Register the client
	h.register <- client

	// Start read and write pumps
	h.wg.Add(2)
	go func() {
		defer h.wg.Done()
		client.writePump(h.ctx)
	}()
	go func() {
		defer h.wg.Done()
		client.readPump(h.ctx)
	}()

	return client, nil
}

// canAcceptConnection checks if a new connection can be accepted based on limits.
func (h *Hub) canAcceptConnection() error {
	if h.limits.MaxConnections > 0 && h.ClientCount() >= h.limits.MaxConnections {
		return ErrMaxConnectionsReached
	}
	return nil
}

// Clients returns all connected clients.
func (h *Hub) Clients() []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	return clients
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClient returns a client by ID.
func (h *Hub) GetClient(id string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.ID == id {
			return client, true
		}
	}
	return nil, false
}

// GetClientByUserID returns a client by user ID.
func (h *Hub) GetClientByUserID(userID string) (*Client, bool) {
	h.userIndexMu.RLock()
	defer h.userIndexMu.RUnlock()

	clients := h.userIndex[userID]
	for client := range clients {
		return client, true // Return first client
	}
	return nil, false
}

// GetClientsByUserID returns all clients for a user ID.
func (h *Hub) GetClientsByUserID(userID string) []*Client {
	h.userIndexMu.RLock()
	defer h.userIndexMu.RUnlock()

	clientMap := h.userIndex[userID]
	clients := make([]*Client, 0, len(clientMap))
	for client := range clientMap {
		clients = append(clients, client)
	}
	return clients
}

// broadcastSequential sends messages to clients sequentially (original method).
func (h *Hub) broadcastSequential(snapshot map[*Client]bool, data []byte) {
	for client := range snapshot {
		select {
		case client.send <- data:
		default:
			h.logger.Warn("Client send buffer full, skipping broadcast",
				"clientID", client.ID,
			)
		}
	}
}

// broadcastParallel sends messages to clients in parallel batches.
func (h *Hub) broadcastParallel(snapshot map[*Client]bool, data []byte) {
	// Convert map to slice for easier batching
	clients := make([]*Client, 0, len(snapshot))
	for client := range snapshot {
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return
	}

	// Calculate number of batches
	batchSize := h.parallelBatchSize
	numBatches := (len(clients) + batchSize - 1) / batchSize

	// Use WaitGroup to wait for all goroutines
	var wg sync.WaitGroup
	wg.Add(numBatches)

	// Fan out to multiple goroutines
	for i := range numBatches {
		start := i * batchSize
		end := min(start + batchSize, len(clients))

		// Launch goroutine for this batch
		go func(batch []*Client) {
			defer wg.Done()

			for _, client := range batch {
				select {
				case client.send <- data:
				default:
					h.logger.Warn("Client send buffer full, skipping broadcast",
						"clientID", client.ID,
					)
					h.metrics.IncrementErrors("send_buffer_full")
				}
			}
		}(clients[start:end])
	}

	// Wait for all batches to complete
	wg.Wait()
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(data []byte) {
	// Load snapshot without any locks!
	snapshot := h.clientsSnapshot.Load().(map[*Client]bool)

	if h.useParallel && len(snapshot) > h.parallelBatchSize {
		h.broadcastParallel(snapshot, data)
	} else {
		h.broadcastSequential(snapshot, data)
	}
}

// BroadcastWithContext sends a message to all clients with context support.
func (h *Hub) BroadcastWithContext(ctx context.Context, data []byte) error {
	snapshot := h.clientsSnapshot.Load().(map[*Client]bool)

	// For context support, we use sequential to check ctx.Done()
	// Parallel version with context would be more complex
	for client := range snapshot {
		select {
		case client.send <- data:
		case <-ctx.Done():
			return ctx.Err()
		default:
			h.logger.Warn("Client send buffer full, skipping broadcast",
				"clientID", client.ID,
			)
		}
	}
	return nil
}

// BroadcastText sends a text message to all connected clients.
func (h *Hub) BroadcastText(text string) {
	h.Broadcast([]byte(text))
}

// BroadcastJSON sends a JSON message to all connected clients.
func (h *Hub) BroadcastJSON(v any) error {
	msg, err := NewJSONMessage(v)
	if err != nil {
		return err
	}
	h.Broadcast(msg.Data)
	return nil
}

// broadcastExceptSequential sends to all clients except one (sequential).
func (h *Hub) broadcastExceptSequential(snapshot map[*Client]bool, data []byte, except *Client) {
	for client := range snapshot {
		if client == except {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
}

// broadcastExceptParallel sends to all clients except one (parallel).
func (h *Hub) broadcastExceptParallel(snapshot map[*Client]bool, data []byte, except *Client) {
	// Filter and convert to slice
	clients := make([]*Client, 0, len(snapshot))
	for client := range snapshot {
		if client != except {
			clients = append(clients, client)
		}
	}

	if len(clients) == 0 {
		return
	}

	batchSize := h.parallelBatchSize
	numBatches := (len(clients) + batchSize - 1) / batchSize

	var wg sync.WaitGroup
	wg.Add(numBatches)

	for i := range numBatches {
		start := i * batchSize
		end := min(start + batchSize, len(clients))

		go func(batch []*Client) {
			defer wg.Done()

			for _, client := range batch {
				select {
				case client.send <- data:
				default:
					h.metrics.IncrementErrors("send_buffer_full")
				}
			}
		}(clients[start:end])
	}

	wg.Wait()
}

// BroadcastExcept sends a message to all clients except the specified one.
func (h *Hub) BroadcastExcept(data []byte, except *Client) {
	snapshot := h.clientsSnapshot.Load().(map[*Client]bool)

	if h.useParallel && len(snapshot) > h.parallelBatchSize {
		h.broadcastExceptParallel(snapshot, data, except)
	} else {
		h.broadcastExceptSequential(snapshot, data, except)
	}
}

// SendToUser sends a message to all clients of a specific user.
func (h *Hub) SendToUser(userID string, data []byte) {
	clients := h.GetClientsByUserID(userID)
	for _, client := range clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// SendToClient sends a message to a specific client by ID.
func (h *Hub) SendToClient(clientID string, data []byte) error {
	client, ok := h.GetClient(clientID)
	if !ok {
		return ErrClientNotFound
	}
	return client.Send(data)
}

// JoinRoom adds a client to a room.
func (h *Hub) JoinRoom(client *Client, roomName string) error {
	// Check if client exists
	h.mu.RLock()
	if _, ok := h.clients[client]; !ok {
		h.mu.RUnlock()
		return ErrClientNotFound
	}
	h.mu.RUnlock()

	// Check room limits
	if h.limits.MaxRoomsPerClient > 0 && client.RoomCount() >= h.limits.MaxRoomsPerClient {
		return ErrMaxRoomsReached
	}

	// Call BeforeRoomJoin hook
	if h.hooks.BeforeRoomJoin != nil {
		if err := h.hooks.BeforeRoomJoin(client, roomName); err != nil {
			return err
		}
	}

	// Get or create room with only roomsMu lock
	h.roomsMu.Lock()
	room, exists := h.rooms[roomName]
	if !exists {
		room = &Room{
			clients: make(map[*Client]bool),
		}
		h.rooms[roomName] = room
	}
	h.roomsMu.Unlock()

	// Lock only this specific room
	room.mu.Lock()
	defer room.mu.Unlock()

	// Check room size limit
	if h.limits.MaxClientsPerRoom > 0 && len(room.clients) >= h.limits.MaxClientsPerRoom {
		return ErrRoomFull
	}

	// Check if already in room
	if room.clients[client] {
		return ErrAlreadyInRoom
	}

	room.clients[client] = true
	client.joinRoom(roomName)

	h.metrics.IncrementRoomJoins()
	h.logger.Debug("Client joined room",
		"clientID", client.ID,
		"room", roomName,
		"roomSize", len(room.clients),
	)

	// Call AfterRoomJoin hook
	if h.hooks.AfterRoomJoin != nil {
		go h.hooks.AfterRoomJoin(client, roomName)
	}

	return nil
}

// LeaveRoom removes a client from a room.
func (h *Hub) LeaveRoom(client *Client, roomName string) error {
	// Get room
	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return ErrRoomNotFound
	}

	// Lock only this room
	room.mu.Lock()
	if !room.clients[client] {
		room.mu.Unlock()
		return ErrNotInRoom
	}

	// Call BeforeRoomLeave hook
	if h.hooks.BeforeRoomLeave != nil {
		h.hooks.BeforeRoomLeave(client, roomName)
	}

	delete(room.clients, client)
	client.leaveRoom(roomName)

	roomEmpty := len(room.clients) == 0
	room.mu.Unlock()

	// Clean up empty room
	if roomEmpty {
		h.roomsMu.Lock()
		// Double-check it's still empty
		room.mu.Lock()
		if len(room.clients) == 0 {
			delete(h.rooms, roomName)
		}
		room.mu.Unlock()
		h.roomsMu.Unlock()
	}

	h.metrics.IncrementRoomLeaves()
	h.logger.Debug("Client left room",
		"clientID", client.ID,
		"room", roomName,
	)

	// Call AfterRoomLeave hook
	if h.hooks.AfterRoomLeave != nil {
		go h.hooks.AfterRoomLeave(client, roomName)
	}

	return nil
}

// LeaveAllRooms removes a client from all rooms.
func (h *Hub) LeaveAllRooms(client *Client) {
	rooms := client.Rooms()

	for _, roomName := range rooms {
		// Get room
		h.roomsMu.RLock()
		room, ok := h.rooms[roomName]
		h.roomsMu.RUnlock()

		if !ok {
			continue
		}

		// Call BeforeRoomLeave hook
		if h.hooks.BeforeRoomLeave != nil {
			h.hooks.BeforeRoomLeave(client, roomName)
		}

		// Lock and remove from room
		room.mu.Lock()
		delete(room.clients, client)
		roomEmpty := len(room.clients) == 0
		room.mu.Unlock()

		client.leaveRoom(roomName)

		// Clean up empty room
		if roomEmpty {
			h.roomsMu.Lock()
			room.mu.Lock()
			if len(room.clients) == 0 {
				delete(h.rooms, roomName)
			}
			room.mu.Unlock()
			h.roomsMu.Unlock()
		}

		h.metrics.IncrementRoomLeaves()

		// Call AfterRoomLeave hook
		if h.hooks.AfterRoomLeave != nil {
			go h.hooks.AfterRoomLeave(client, roomName)
		}
	}

	client.mu.Lock()
	client.rooms = make(map[string]bool)
	client.mu.Unlock()
}

// broadcastToRoomSequential sends to all clients in a room (sequential).
func (h *Hub) broadcastToRoomSequential(room *Room, data []byte) {
	room.mu.RLock()
	defer room.mu.RUnlock()

	for client := range room.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// broadcastToRoomParallel sends to all clients in a room (parallel).
func (h *Hub) broadcastToRoomParallel(room *Room, data []byte) {
	room.mu.RLock()
	clients := make([]*Client, 0, len(room.clients))
	for client := range room.clients {
		clients = append(clients, client)
	}
	room.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	batchSize := h.parallelBatchSize
	numBatches := (len(clients) + batchSize - 1) / batchSize

	var wg sync.WaitGroup
	wg.Add(numBatches)

	for i := range numBatches {
		start := i * batchSize
		end := min(start + batchSize, len(clients))

		go func(batch []*Client) {
			defer wg.Done()

			for _, client := range batch {
				select {
				case client.send <- data:
				default:
					h.metrics.IncrementErrors("send_buffer_full")
				}
			}
		}(clients[start:end])
	}

	wg.Wait()
}

// BroadcastToRoom sends a message to all clients in a room.
// OPTIMIZATION #2: Locks only the specific room
// OPTIMIZATION #4: Uses parallel fan-out if enabled.
func (h *Hub) BroadcastToRoom(roomName string, data []byte) error {
	// Get room with minimal lock time
	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return ErrRoomNotFound
	}

	if h.useParallel {
		// Count clients to decide if parallel is worth it
		room.mu.RLock()
		clientCount := len(room.clients)
		room.mu.RUnlock()

		if clientCount > h.parallelBatchSize {
			h.broadcastToRoomParallel(room, data)
			return nil
		}
	}

	h.broadcastToRoomSequential(room, data)
	return nil
}

// BroadcastToRoomExcept sends a message to all clients in a room except one.
// OPTIMIZATION #2: Locks only the specific room
func (h *Hub) BroadcastToRoomExcept(roomName string, data []byte, except *Client) error {
	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return ErrRoomNotFound
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	for client := range room.clients {
		if client == except {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}

	return nil
}

// RoomClients returns all clients in a room.
func (h *Hub) RoomClients(roomName string) []*Client {
	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return nil
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	clients := make([]*Client, 0, len(room.clients))
	for client := range room.clients {
		clients = append(clients, client)
	}
	return clients
}

// RoomCount returns the number of clients in a room.
func (h *Hub) RoomCount(roomName string) int {
	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return 0
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	return len(room.clients)
}

// RoomNames returns all room names.
func (h *Hub) RoomNames() []string {
	h.roomsMu.RLock()
	defer h.roomsMu.RUnlock()

	rooms := make([]string, 0, len(h.rooms))
	for room := range h.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// RoomExists checks if a room exists.
func (h *Hub) RoomExists(roomName string) bool {
	h.roomsMu.RLock()
	defer h.roomsMu.RUnlock()
	_, ok := h.rooms[roomName]
	return ok
}
