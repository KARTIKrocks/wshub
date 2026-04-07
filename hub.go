package wshub

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// registrationResult is sent back from the Run goroutine to UpgradeConnection
// to indicate whether the client was accepted.
type registrationResult struct {
	client *Client
	err    error
}

// registrationRequest is sent to the Run goroutine to register a new client.
type registrationRequest struct {
	client *Client
	result chan<- registrationResult
}

// HubState represents the lifecycle state of a [Hub].
type HubState int32

const (
	// StateRunning means the hub is accepting new connections and processing
	// messages normally.
	StateRunning HubState = iota

	// StateDraining means the hub has stopped accepting new connections
	// (returning HTTP 503) but is allowing existing connections to finish
	// their in-flight messages. Initiated by [Hub.Drain].
	StateDraining

	// StateStopped means the hub has been shut down via [Hub.Shutdown].
	// All connections are closed and the Run loop has exited.
	StateStopped
)

// String returns a human-readable name for the hub state.
func (s HubState) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateDraining:
		return "draining"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// DropPolicy controls what happens when a client's send buffer is full.
type DropPolicy int

const (
	// DropNewest drops the new message when the send buffer is full.
	// This is the default and matches the original behavior.
	DropNewest DropPolicy = iota

	// DropOldest evicts the oldest queued message to make room for the new one.
	// This ensures the client always receives the most recent data, at the cost
	// of losing older messages.
	DropOldest
)

// hubSnapshot holds both a map and a pre-built slice of clients so that
// broadcast paths can use the slice directly without allocating on every call.
type hubSnapshot struct {
	set   map[*Client]struct{} // O(1) membership checks
	slice []*Client            // pre-built for parallelSend / iteration
}

// Room represents a chat room with its own lock for better concurrency.
type Room struct {
	mu       sync.RWMutex
	clients  map[*Client]struct{}
	snapshot atomic.Value // []*Client — lock-free snapshot for broadcasts
}

// Hub maintains the set of active clients and broadcasts messages.
//
// Lock ordering (acquire in this order to prevent deadlocks):
//
//	mu (hub clients) → roomsMu → Room.mu → Client.mu → userIndexMu
//
// Not all paths acquire every lock; the rule is that when multiple locks
// from this list are held simultaneously, the earlier one must be acquired
// first. Individual locks that are never held together (e.g. Client.mu and
// userIndexMu acquired sequentially, not nested) are safe regardless of order.
type Hub struct {
	// Clients is the set of registered clients.
	clients map[*Client]struct{}

	// O(1) client lookup by ID
	clientIndex map[string]*Client

	// Lock-free snapshot for broadcasting
	clientsSnapshot atomic.Value // hubSnapshot

	// Atomic client count — avoids locking h.mu for ClientCount().
	clientCount atomic.Int64

	// Rooms with per-room locks
	rooms   map[string]*Room
	roomsMu sync.RWMutex

	// roomVersion is incremented on every room join/leave to allow
	// presence publishing to detect changes in O(1).
	roomVersion atomic.Int64

	// User ID index for O(1) lookups
	userIndex   map[string]map[*Client]struct{} // userID -> clients
	userIndexMu sync.RWMutex

	// Channels for client management.
	register   chan registrationRequest
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

	// Hub lifecycle state — lock-free reads from UpgradeConnection hot path.
	state atomic.Int32 // HubState stored as int32

	// alive is 1 while the Run() goroutine is executing, 0 otherwise.
	// Set at the top of Run(), cleared when Run() exits via defer.
	alive atomic.Int32

	// startedAt records when Run() began executing. Zero value until Run() starts.
	startedAt atomic.Value // time.Time

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Drain signaling — drainDone is closed when all clients have disconnected
	// during drain, or when Shutdown forces completion. Allocated eagerly in
	// NewHub to avoid data races between Drain() and handleUnregister/Shutdown.
	drainDone chan struct{}
	drainOnce sync.Once

	// drainTimeout controls how long an idle connection can remain open after
	// Drain() is called. Connections whose send buffers have been empty for
	// this duration are proactively closed with CloseGoingAway (1001).
	// Default: 30s. Set to 0 via WithDrainTimeout to disable the idle reaper.
	drainTimeout time.Duration

	// Parallel broadcast configuration
	parallelBatchSize int  // Number of clients per goroutine (default: 100)
	useParallel       bool // Enable parallel broadcasting (default: false, enable for 1000+ clients)

	// Worker pool for parallel broadcast (nil until initialized via ensurePool).
	pool     *workerPool
	poolOnce sync.Once
	poolSize int // number of worker goroutines (default: runtime.NumCPU())

	// Drop policy for full send buffers (default: DropNewest)
	dropPolicy DropPolicy

	// skipHandlerLatency disables the automatic RecordLatency call in
	// processMessage. Set via WithoutHandlerLatency to avoid
	// double-counting when MetricsMiddleware is used in the handler chain.
	skipHandlerLatency bool

	// Multi-node adapter for cross-node message delivery (nil = single-node mode).
	adapter Adapter

	// nodeID uniquely identifies this hub instance for adapter message deduplication.
	nodeID string

	// hookTimeout is the maximum time to wait for synchronous hooks
	// (e.g. BeforeDisconnect) before proceeding. Default: 5s.
	hookTimeout time.Duration

	// Presence: periodic stats gossip for global counts (nil cache = disabled).
	presenceMu       sync.RWMutex
	presenceCache    map[string]*nodeStats // nodeID -> cached stats (nil = presence disabled)
	presenceInterval time.Duration
	presenceTTL      time.Duration
}

// NewHub creates a new WebSocket hub.
func NewHub(opts ...Option) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	h := &Hub{
		clients:           make(map[*Client]struct{}),
		clientIndex:       make(map[string]*Client),
		rooms:             make(map[string]*Room),
		userIndex:         make(map[string]map[*Client]struct{}),
		register:          make(chan registrationRequest, 64),
		unregister:        make(chan *Client, 64),
		config:            DefaultConfig(),
		limits:            DefaultLimits(),
		logger:            &NoOpLogger{},
		metrics:           &NoOpMetrics{},
		ctx:               ctx,
		cancel:            cancel,
		parallelBatchSize: 100,
		useParallel:       false,
		poolSize:          runtime.NumCPU(),
		hookTimeout:       5 * time.Second,
		drainDone:         make(chan struct{}),
		drainTimeout:      30 * time.Second,
		nodeID:            uuid.New().String(),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(h)
	}

	// Validate config before applying defaults so we can warn about
	// user-specified values that will be auto-corrected.
	for _, w := range validateConfig(h.config) {
		h.logger.Warn(w)
	}

	// Fill zero-value config fields with defaults so that a partial
	// Config{} literal behaves the same as DefaultConfig() for unset fields.
	h.config = applyConfigDefaults(h.config)

	// Build upgrader from final config
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:    h.config.ReadBufferSize,
		WriteBufferSize:   h.config.WriteBufferSize,
		CheckOrigin:       h.config.CheckOrigin,
		EnableCompression: h.config.EnableCompression,
		Subprotocols:      h.config.Subprotocols,
	}

	// Warn if the origin checker accepts all origins. This is convenient
	// for development but a security risk in production.
	if h.config.CheckOrigin != nil {
		probe := &http.Request{Header: http.Header{"Origin": []string{"https://attacker.example.com"}}}
		probe.Host = "legitimate.example.com"
		if h.config.CheckOrigin(probe) {
			h.logger.Warn("CheckOrigin allows all origins — restrict this in production")
		}
	}

	// Initialize snapshot with empty hubSnapshot
	h.clientsSnapshot.Store(hubSnapshot{set: make(map[*Client]struct{}), slice: nil})

	// Initialize presence cache only when both presence and adapter are
	// configured. Done here (after all options) rather than in WithPresence
	// to avoid allocating when no adapter is set, and before Run() to avoid
	// racing with the adapter subscription handler.
	if h.adapter != nil && h.presenceInterval > 0 {
		h.presenceCache = make(map[string]*nodeStats)
	}

	// Pre-add to WaitGroup so Shutdown().wg.Wait() blocks even if Run()
	// hasn't started yet, eliminating a race between go hub.Run() and
	// hub.Shutdown().
	h.wg.Add(1)

	return h
}

// emptySnapshot is returned by loadSnapshot when the atomic.Value has not
// been initialized yet. Using a package-level variable avoids allocation
// on every call.
var emptySnapshot = hubSnapshot{set: map[*Client]struct{}{}, slice: nil}

// emptyRoomSnapshot is returned by loadRoomSnapshot when the room's
// atomic.Value has not been initialized yet.
var emptyRoomSnapshot = []*Client{}

// loadSnapshot returns the current lock-free client snapshot. It uses the
// comma-ok type assertion to avoid panicking if the atomic.Value holds an
// unexpected type.
func (h *Hub) loadSnapshot() hubSnapshot {
	snap, ok := h.clientsSnapshot.Load().(hubSnapshot)
	if !ok {
		return emptySnapshot
	}
	return snap
}

// updateClientsSnapshot creates a new snapshot of clients for lock-free reads.
// Called exclusively from drainAndRebuildSnapshot in the single-threaded Run
// goroutine — the only goroutine that writes to h.clients — so no lock is
// needed for the copy. Concurrent readers (GetClient, etc.) only take RLock
// which is compatible with concurrent map reads.
func (h *Hub) updateClientsSnapshot() {
	set := make(map[*Client]struct{}, len(h.clients))
	slice := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		set[client] = struct{}{}
		slice = append(slice, client)
	}
	h.clientsSnapshot.Store(hubSnapshot{set: set, slice: slice})
}

// loadRoomSnapshot returns the current lock-free client slice for a room.
func loadRoomSnapshot(room *Room) []*Client {
	snapshot, _ := room.snapshot.Load().([]*Client)
	if snapshot == nil {
		return emptyRoomSnapshot
	}
	return snapshot
}

// rebuildRoomSnapshot creates a new snapshot slice from room.clients and
// stores it atomically. Must be called while room.mu is held (read or write).
func rebuildRoomSnapshot(room *Room) {
	clients := make([]*Client, 0, len(room.clients))
	for client := range room.clients {
		clients = append(clients, client)
	}
	room.snapshot.Store(clients)
}

// addToUserIndex adds a client to the user index, checking
// MaxConnectionsPerUser before inserting. Returns an error if the
// per-user limit would be exceeded.
func (h *Hub) addToUserIndex(client *Client) error {
	userID := client.GetUserID()
	if userID == "" {
		return nil
	}

	h.userIndexMu.Lock()
	defer h.userIndexMu.Unlock()

	if h.limits.MaxConnectionsPerUser > 0 && len(h.userIndex[userID]) >= h.limits.MaxConnectionsPerUser {
		return ErrMaxUserConnectionsReached
	}

	if h.userIndex[userID] == nil {
		h.userIndex[userID] = make(map[*Client]struct{})
	}
	h.userIndex[userID][client] = struct{}{}
	return nil
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

// setClientUserID atomically reads the client's current user ID, checks the
// per-user limit, updates the user index, and sets the new user ID — all under
// userIndexMu — eliminating the TOCTOU race in SetUserID.
func (h *Hub) setClientUserID(client *Client, newUserID string) error {
	h.userIndexMu.Lock()
	defer h.userIndexMu.Unlock()

	client.mu.Lock()
	oldUserID := client.userID
	if oldUserID == newUserID {
		client.mu.Unlock()
		return nil
	}

	// Check per-user limit under the same lock that will perform the insert.
	if h.limits.MaxConnectionsPerUser > 0 && newUserID != "" {
		if len(h.userIndex[newUserID]) >= h.limits.MaxConnectionsPerUser {
			client.mu.Unlock()
			return ErrMaxUserConnectionsReached
		}
	}

	client.userID = newUserID
	client.mu.Unlock()

	// Update index.
	if oldUserID != "" {
		if clients, ok := h.userIndex[oldUserID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.userIndex, oldUserID)
			}
		}
	}
	if newUserID != "" {
		if h.userIndex[newUserID] == nil {
			h.userIndex[newUserID] = make(map[*Client]struct{})
		}
		h.userIndex[newUserID][client] = struct{}{}
	}
	return nil
}

// NodeID returns this hub's unique node identifier.
// In multi-node setups each hub has a distinct ID used for message deduplication.
func (h *Hub) NodeID() string {
	return h.nodeID
}

// State returns the current lifecycle state of the hub.
// Safe for concurrent use; the read is a single atomic load.
func (h *Hub) State() HubState {
	return HubState(h.state.Load())
}

// IsDraining reports whether the hub is in the draining state.
func (h *Hub) IsDraining() bool {
	return h.State() == StateDraining
}

// IsRunning reports whether the hub is in the running state and accepting
// new connections. Returns false when draining or stopped.
func (h *Hub) IsRunning() bool {
	return h.State() == StateRunning
}

// ensurePool lazily initializes the worker pool on first use.
// This allows benchmarks that set useParallel directly (without calling Run)
// to work correctly.
func (h *Hub) ensurePool() *workerPool {
	h.poolOnce.Do(func() {
		h.pool = newWorkerPool(h.poolSize)
	})
	return h.pool
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	// wg.Add(1) is done in NewHub so Shutdown can safely call wg.Wait()
	// even before Run starts.
	defer h.wg.Done()

	h.alive.Store(1)
	defer h.alive.Store(0)
	h.startedAt.Store(time.Now())

	// Start worker pool for parallel broadcasts if enabled.
	if h.useParallel {
		h.ensurePool()
	}

	// Start adapter subscription if configured.
	if h.adapter != nil {
		if err := h.adapter.Subscribe(h.ctx, h.handleAdapterMessage); err != nil {
			h.logger.Error("adapter subscribe failed", "error", err)
			h.metrics.IncrementErrors("adapter_subscribe")
		}
	}

	// Start presence gossip if enabled (cache was initialized in NewHub).
	if h.adapter != nil && h.presenceInterval > 0 {
		h.wg.Add(1)
		go h.runPresence()
	}

	for {
		select {
		case <-h.ctx.Done():
			// Shutdown: close all client connections
			h.mu.Lock()
			for client := range h.clients {
				_ = client.Close()
			}
			h.mu.Unlock()
			h.logger.Info("Hub shutdown complete")
			return

		case req := <-h.register:
			h.handleRegister(req)
			h.drainAndRebuildSnapshot()

		case client := <-h.unregister:
			h.handleUnregister(client)
			h.drainAndRebuildSnapshot()
		}
	}
}

// handleRegister processes a single client registration.
// The limit check runs here inside the Run goroutine, eliminating the
// TOCTOU race that existed when it was checked in UpgradeConnection.
func (h *Hub) handleRegister(req registrationRequest) {
	client := req.client

	// Authoritative connection-limit check (single-threaded in Run).
	if h.limits.MaxConnections > 0 && len(h.clients) >= h.limits.MaxConnections {
		req.result <- registrationResult{err: ErrMaxConnectionsReached}
		return
	}

	// Check per-user connection limit and add to user index atomically.
	// This enforces MaxConnectionsPerUser for clients created with
	// WithUserID, which sets the user ID before registration.
	if err := h.addToUserIndex(client); err != nil {
		req.result <- registrationResult{err: err}
		return
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.clientIndex[client.ID] = client
	h.mu.Unlock()
	h.clientCount.Add(1)

	h.metrics.IncrementConnections()
	h.logger.Info("Client registered",
		"clientID", client.ID,
		"totalClients", h.ClientCount(),
	)

	// Signal the caller that registration succeeded.
	req.result <- registrationResult{client: client}

	if h.hooks.AfterConnect != nil {
		go h.hooks.AfterConnect(client)
	}
}

// handleUnregister processes a single client unregistration.
func (h *Hub) handleUnregister(client *Client) {
	// Call BeforeDisconnect hook while the client is still fully registered,
	// so the hook can inspect rooms, user index, etc. Run in a goroutine
	// with a timeout to avoid blocking the Run() loop if the hook is slow.
	if h.hooks.BeforeDisconnect != nil {
		done := make(chan struct{})
		go func() {
			defer close(done)
			h.hooks.BeforeDisconnect(client)
		}()
		timer := time.NewTimer(h.hookTimeout)
		select {
		case <-done:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			h.logger.Warn("BeforeDisconnect hook timed out",
				"clientID", client.ID,
			)
		}
	}

	// Check presence under h.mu but don't remove yet — we want the
	// user-index and room cleanup to happen first so that concurrent
	// lookups (GetClientsByUserID, RoomClients) never return a client
	// that has already been removed from h.clients.
	h.mu.RLock()
	_, present := h.clients[client]
	h.mu.RUnlock()

	if !present {
		return
	}

	// Remove from secondary indexes before the primary map so that
	// concurrent readers see a consistent state.
	h.removeFromUserIndex(client)
	h.removeClientFromAllRoomsWithHooks(client)

	h.mu.Lock()
	delete(h.clients, client)
	delete(h.clientIndex, client.ID)
	h.mu.Unlock()

	h.clientCount.Add(-1)

	// If draining and this was the last client, signal drain completion.
	if h.State() == StateDraining && h.clientCount.Load() == 0 {
		h.drainOnce.Do(func() {
			close(h.drainDone)
		})
	}

	// Mark the client as closed so IsClosed() returns the correct value
	// and SendMessage short-circuits. Track whether CloseWithCode already
	// closed the send channel so we can skip the drain below.
	client.mu.Lock()
	sendChanClosed := client.closed
	if !sendChanClosed {
		client.closed = true
		client.closedAt = time.Now()
	}
	client.mu.Unlock()

	// Signal writePump to exit immediately via the done channel, then
	// kill the underlying connection so any in-flight write fails.
	client.closeDone()
	client.closeConn()

	// Drain any messages left in the send buffer to free memory
	// immediately rather than waiting for GC. We drain instead of
	// close(client.send) to avoid a data race with concurrent senders
	// in trySendErr that have already passed the closed check.
	//
	// Skip when CloseWithCode already closed the channel — receiving
	// from a closed channel always succeeds with the zero value, which
	// would spin this loop forever and hang the Run goroutine.
	if !sendChanClosed {
	drain:
		for {
			select {
			case <-client.send:
			default:
				break drain
			}
		}
	}

	h.metrics.DecrementConnections()
	h.logger.Info("Client unregistered",
		"clientID", client.ID,
		"totalClients", h.ClientCount(),
	)

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

// drainAndRebuildSnapshot drains any pending register/unregister events,
// then rebuilds the clients snapshot once. During connection bursts this
// coalesces N map copies into 1.
func (h *Hub) drainAndRebuildSnapshot() {
	for {
		select {
		case req := <-h.register:
			h.handleRegister(req)
		case client := <-h.unregister:
			h.handleUnregister(client)
		default:
			h.updateClientsSnapshot()
			return
		}
	}
}

// removeClientFromAllRoomsWithHooks removes a client from all rooms,
// firing BeforeRoomLeave/AfterRoomLeave hooks so that disconnect behaves
// consistently with explicit LeaveRoom/LeaveAllRooms calls.
//
// Each room is processed individually so that roomsMu is NOT held while
// hooks execute — preventing deadlocks when hooks call Hub methods that
// acquire roomsMu (e.g. RoomNames, RoomCount).
func (h *Hub) removeClientFromAllRoomsWithHooks(client *Client) {
	// Snapshot client.rooms under lock to avoid data race.
	client.mu.RLock()
	roomNames := make([]string, 0, len(client.rooms))
	for room := range client.rooms {
		roomNames = append(roomNames, room)
	}
	client.mu.RUnlock()

	if len(roomNames) == 0 {
		return
	}

	var leftRooms []string
	for _, name := range roomNames {
		h.roomsMu.RLock()
		room, ok := h.rooms[name]
		h.roomsMu.RUnlock()
		if !ok {
			continue
		}

		// Check membership before calling the hook so the hook runs
		// without room.mu held, preventing deadlocks when the hook
		// queries the same room (e.g. RoomCount, RoomClients).
		room.mu.RLock()
		_, inRoom := room.clients[client]
		room.mu.RUnlock()
		if !inRoom {
			continue
		}

		if h.hooks.BeforeRoomLeave != nil {
			h.hooks.BeforeRoomLeave(client, name)
		}

		// Re-check under write lock — client may have left concurrently.
		room.mu.Lock()
		if _, inRoom := room.clients[client]; !inRoom {
			room.mu.Unlock()
			continue
		}
		delete(room.clients, client)
		rebuildRoomSnapshot(room)
		roomEmpty := len(room.clients) == 0
		room.mu.Unlock()

		if roomEmpty {
			h.deleteRoomIfEmpty(name, room)
		}

		leftRooms = append(leftRooms, name)
	}

	// Clean up client.rooms so Rooms() returns accurate data in
	// AfterDisconnect / onClose callbacks.
	client.mu.Lock()
	for _, name := range leftRooms {
		delete(client.rooms, name)
	}
	client.mu.Unlock()

	h.roomVersion.Add(int64(len(leftRooms)))

	// Record metrics for each room leave.
	for range leftRooms {
		h.metrics.IncrementRoomLeaves()
	}

	// Fire AfterRoomLeave hooks after modifying state.
	for _, name := range leftRooms {
		if h.hooks.AfterRoomLeave != nil {
			go h.hooks.AfterRoomLeave(client, name)
		}
	}
}

// deleteRoomIfEmpty removes a room from h.rooms if it has no clients.
// It acquires roomsMu and room.mu in the documented lock order and
// double-checks emptiness to handle concurrent joins.
func (h *Hub) deleteRoomIfEmpty(roomName string, room *Room) {
	h.roomsMu.Lock()
	room.mu.Lock()
	if len(room.clients) == 0 && h.rooms[roomName] == room {
		delete(h.rooms, roomName)
		h.metrics.DecrementRooms()
	}
	room.mu.Unlock()
	h.roomsMu.Unlock()
}

// Drain initiates graceful connection draining. It stops accepting new
// connections ([Hub.UpgradeConnection] returns HTTP 503) and waits for all
// existing connections to disconnect or for the context to expire.
//
// During drain, idle connections whose send buffers have been empty for the
// configured drain timeout (see [WithDrainTimeout]) are proactively closed
// with CloseGoingAway (1001). This prevents indefinite waiting for clients
// that are connected but doing nothing.
//
// Drain returns nil when all clients have disconnected, or the context
// error if the context expires first. Calling Drain on an already-draining
// hub blocks on the same completion signal. Calling Drain after [Hub.Shutdown]
// returns immediately.
//
// Typical usage in a Kubernetes preStop hook:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	hub.Drain(ctx)    // stop new connections, wait for existing ones
//	hub.Shutdown(ctx) // force-close anything remaining
func (h *Hub) Drain(ctx context.Context) error {
	// Transition Running → Draining via CAS. If not Running, handle accordingly.
	if !h.state.CompareAndSwap(int32(StateRunning), int32(StateDraining)) {
		current := h.State()
		if current == StateStopped {
			return nil // already shut down
		}
		// current == StateDraining: fall through to wait on existing drainDone.
	} else {
		// We won the CAS — we are the drain initiator.
		h.logger.Info("Hub drain initiated", "clients", h.ClientCount())

		// If there are no clients, complete immediately.
		if h.clientCount.Load() == 0 {
			h.drainOnce.Do(func() { close(h.drainDone) })
			h.logger.Info("Hub drain complete")
			return nil
		}

		// Start the idle connection reaper if configured.
		if h.drainTimeout > 0 {
			h.wg.Add(1)
			go h.runDrainReaper()
		}
	}

	// Wait for all clients to disconnect or context to expire.
	select {
	case <-h.drainDone:
		h.logger.Info("Hub drain complete")
		return nil
	case <-ctx.Done():
		h.logger.Warn("Hub drain timeout", "remainingClients", h.ClientCount())
		return ctx.Err()
	}
}

// runDrainReaper periodically closes idle connections during drain.
// A connection is considered idle when its send buffer has been continuously
// empty for the entire drain timeout duration.
func (h *Hub) runDrainReaper() {
	defer h.wg.Done()

	// Check interval: drain timeout / 4, but at least 100ms to avoid
	// busy-spinning. The lower bound allows short drain timeouts (e.g. in
	// tests) to be responsive without requiring two full seconds of ticking.
	interval := max(h.drainTimeout/4, 100*time.Millisecond)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Track when each client's send buffer was first observed empty.
	// This map is owned exclusively by this goroutine — no lock needed.
	firstIdleAt := make(map[*Client]time.Time)

	for {
		select {
		case <-h.drainDone:
			return
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			snap := h.loadSnapshot()

			// Remove entries for clients no longer in the snapshot.
			for c := range firstIdleAt {
				if _, ok := snap.set[c]; !ok {
					delete(firstIdleAt, c)
				}
			}

			for _, client := range snap.slice {
				if len(client.send) == 0 {
					// Send buffer empty — record first-seen-idle time.
					if _, tracked := firstIdleAt[client]; !tracked {
						firstIdleAt[client] = now
					} else if now.Sub(firstIdleAt[client]) >= h.drainTimeout {
						h.logger.Info("Closing idle connection during drain",
							"clientID", client.ID,
						)
						_ = client.CloseWithCode(websocket.CloseGoingAway, "server draining")
						delete(firstIdleAt, client)
					}
				} else {
					// Client has pending messages — reset idle tracking.
					delete(firstIdleAt, client)
				}
			}
		}
	}
}

// Shutdown gracefully shuts down the hub. It force-closes all remaining
// connections and waits for goroutines to exit. If the hub is draining,
// Shutdown unblocks any pending [Hub.Drain] call.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.logger.Info("Shutting down hub")

	// Set state to Stopped. This rejects new connections and signals
	// any pending Drain() to unblock.
	h.state.Store(int32(StateStopped))

	// Close drainDone to unblock any pending Drain() call.
	h.drainOnce.Do(func() { close(h.drainDone) })

	h.cancel()

	// Close the adapter before waiting on goroutines — the subscriber
	// goroutine may block on a channel read that only unblocks on close.
	if h.adapter != nil {
		if err := h.adapter.Close(); err != nil {
			h.logger.Error("adapter close failed", "error", err)
		}
	}

	// Stop the worker pool if it was started. This drains remaining tasks
	// before workers exit, ensuring in-flight broadcasts complete.
	// Synchronize via poolOnce to ensure visibility of any pool created
	// by a concurrent ensurePool call (e.g. Run still starting up).
	h.poolOnce.Do(func() {})
	if h.pool != nil {
		h.pool.shutdown()
	}

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

// UpgradeOption configures a single UpgradeConnection call.
type UpgradeOption func(*Client)

// WithUserID sets the user ID on the client atomically during connection
// upgrade, before the client is registered. This avoids the window where
// a client exists without a user ID, which can bypass MaxConnectionsPerUser.
func WithUserID(userID string) UpgradeOption {
	return func(c *Client) {
		c.userID = userID
	}
}

// HandleHTTP returns an HTTP handler that upgrades connections to WebSocket.
// Upgrade errors are logged via the hub's logger.
func (h *Hub) HandleHTTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := h.UpgradeConnection(w, r); err != nil {
			h.logger.Error("WebSocket upgrade failed", "error", err, "remote", r.RemoteAddr)
		}
	}
}

// UpgradeConnection upgrades an HTTP connection to WebSocket.
func (h *Hub) UpgradeConnection(w http.ResponseWriter, r *http.Request, opts ...UpgradeOption) (*Client, error) {
	// Reject connections when Run() has not been called yet.
	if !h.Alive() {
		h.metrics.IncrementErrors("connection_rejected_not_ready")
		http.Error(w, "Service not ready", http.StatusServiceUnavailable)
		return nil, fmt.Errorf("remote %s: %w", r.RemoteAddr, ErrHubNotStarted)
	}

	// Reject connections when the hub is not running (draining or stopped).
	// Checked before BeforeConnect hook to avoid unnecessary work.
	if s := h.State(); s != StateRunning {
		h.metrics.IncrementErrors("connection_rejected_draining")
		http.Error(w, "Service draining", http.StatusServiceUnavailable)
		if s == StateDraining {
			return nil, fmt.Errorf("remote %s: %w", r.RemoteAddr, ErrHubDraining)
		}
		return nil, fmt.Errorf("remote %s: %w", r.RemoteAddr, ErrHubStopped)
	}

	// Call BeforeConnect hook
	if h.hooks.BeforeConnect != nil {
		if err := h.hooks.BeforeConnect(r); err != nil {
			h.logger.Warn("Connection rejected by BeforeConnect hook", "error", err)
			h.metrics.IncrementErrors("connection_rejected")
			http.Error(w, "Connection rejected", http.StatusForbidden)
			return nil, err
		}
	}

	// Fast-path connection limit check. This is non-authoritative (the
	// authoritative check runs in handleRegister), but it lets us reject
	// most over-limit connections before performing the WebSocket upgrade.
	if h.limits.MaxConnections > 0 && int(h.clientCount.Load()) >= h.limits.MaxConnections {
		h.logger.Warn("Connection limit reached")
		h.metrics.IncrementErrors("connection_limit")
		http.Error(w, ErrMaxConnectionsReached.Error(), http.StatusServiceUnavailable)
		return nil, fmt.Errorf("remote %s: %w", r.RemoteAddr, ErrMaxConnectionsReached)
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection", "error", err)
		h.metrics.IncrementErrors("upgrade_failed")
		return nil, err
	}

	client := newClient(h, conn, h.config, r)

	// Apply upgrade options (e.g., WithUserID) before registration
	// so that limit checks see the final state.
	for _, opt := range opts {
		opt(client)
	}

	// writeCloseAndClose sends a WebSocket close frame with the given code and
	// reason, then closes the underlying connection. Used in the pre-registration
	// path where no Client exists yet.
	writeCloseAndClose := func(code int, reason string) {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
		_ = conn.Close()
	}

	// Register the client via the Run goroutine which checks limits
	// atomically, eliminating the TOCTOU race.
	result := make(chan registrationResult, 1)
	select {
	case h.register <- registrationRequest{client: client, result: result}:
	case <-h.ctx.Done():
		writeCloseAndClose(websocket.CloseGoingAway, "server shutting down")
		return nil, h.ctx.Err()
	}

	var res registrationResult
	select {
	case res = <-result:
	case <-h.ctx.Done():
		writeCloseAndClose(websocket.CloseGoingAway, "server shutting down")
		return nil, h.ctx.Err()
	}
	if res.err != nil {
		h.logger.Warn("Connection limit reached")
		h.metrics.IncrementErrors("connection_limit")
		writeCloseAndClose(websocket.CloseTryAgainLater, res.err.Error())
		return nil, fmt.Errorf("remote %s: %w", r.RemoteAddr, res.err)
	}

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

// trySend is a convenience wrapper around trySendErr that discards the
// return value. Used by broadcast paths where individual send failures
// are expected (client disconnecting) and do not need per-call handling.
func (h *Hub) trySend(client *Client, item sendItem) {
	h.trySendErr(client, item)
}

// trySendErr is the core send path. It returns true if the message was
// enqueued, false if it was dropped. The behavior when the buffer is full
// depends on the hub's DropPolicy:
//
//   - DropNewest (default): the new message is discarded.
//   - DropOldest: the oldest queued message is evicted to make room.
//     The evict+enqueue sequence is serialized per-client via sendMu
//     to prevent concurrent writers from losing both messages.
//
// A recover guard protects against sending on a closed channel if the
// client disconnects concurrently.
func (h *Hub) trySendErr(client *Client, item sendItem) (ok bool) {
	// Quick check: skip clients that are already closed to avoid
	// unnecessary panic+recover on the closed channel.
	client.mu.RLock()
	closed := client.closed
	client.mu.RUnlock()
	if closed {
		return false
	}

	defer func() {
		if r := recover(); r != nil {
			if isChanSendPanic(r) {
				ok = false // client.send was closed — the client is disconnecting.
				return
			}
			panic(r) // re-panic for unexpected errors
		}
	}()

	// Fast path: try non-blocking send.
	select {
	case client.send <- item:
		return true
	default:
	}

	// Buffer is full — apply drop policy.
	switch h.dropPolicy {
	case DropOldest:
		if h.trySendDropOldest(client, item) {
			return true
		}

	default:
		// DropNewest: discard the new message.
	}

	h.notifySendDropped(client, item.data)
	return false
}

// trySendDropOldest is the DropOldest sub-path of trySendErr. It
// serializes evict+enqueue via sendMu and uses defer to guarantee the
// mutex is released even if a send-on-closed-channel panic propagates
// through to the recover guard in trySendErr.
func (h *Hub) trySendDropOldest(client *Client, item sendItem) bool {
	client.sendMu.Lock()
	defer client.sendMu.Unlock()

	// Re-check: buffer may have drained while we waited for the lock.
	select {
	case client.send <- item:
		return true
	default:
	}

	// Evict+enqueue loop: retry up to 2 times to avoid losing both the
	// evicted message and the new message when a fast-path writer races
	// us between evict and enqueue.
	for range 2 {
		// Evict the oldest message.
		select {
		case dropped := <-client.send:
			h.notifySendDropped(client, dropped.data)
		default:
			// Drained concurrently by writePump — buffer now has space.
		}

		// Enqueue the new message.
		select {
		case client.send <- item:
			return true
		default:
			// Fast-path writer filled the slot; retry evict+enqueue.
		}
	}
	return false
}

// notifySendDropped logs, counts, and fires the OnSendDropped hook.
func (h *Hub) notifySendDropped(client *Client, data []byte) {
	h.logger.Warn("Client send buffer full, message dropped",
		"clientID", client.ID,
	)
	h.metrics.IncrementMessagesDropped()

	if h.hooks.OnSendDropped != nil {
		h.hooks.OnSendDropped(client, data)
	}
}

// publishToAdapter publishes an adapter message to other nodes.
// It is a no-op when no adapter is configured. Errors are logged and
// counted via metrics — local delivery is never blocked by adapter failures.
func (h *Hub) publishToAdapter(msg AdapterMessage) {
	if h.adapter == nil {
		return
	}
	msg.NodeID = h.nodeID
	if err := h.adapter.Publish(h.ctx, msg); err != nil {
		h.logger.Error("adapter publish failed", "error", err, "type", msg.Type)
		h.metrics.IncrementErrors("adapter_publish")
	}
}

// handleAdapterMessage processes a message received from another node.
// It dispatches locally only — never re-publishes — preventing infinite loops.
func (h *Hub) handleAdapterMessage(msg AdapterMessage) {
	// Ignore messages originating from this node.
	if msg.NodeID == h.nodeID {
		return
	}

	item := sendItem{msgType: msg.MsgType, data: msg.Data}

	switch msg.Type {
	case AdapterBroadcast:
		h.broadcast(item)

	case AdapterBroadcastExcept:
		h.broadcastExceptByIDs(item, msg.ExceptClientIDs)

	case AdapterRoom:
		h.broadcastToRoomLocal(msg.Room, item)

	case AdapterRoomExcept:
		h.broadcastToRoomExceptByIDs(msg.Room, item, msg.ExceptClientIDs)

	case AdapterUser:
		h.sendToUserLocal(msg.UserID, item)

	case AdapterClient:
		_ = h.sendToClientLocal(msg.ClientID, msg.Data, msg.MsgType)

	case AdapterPresence:
		h.handlePresenceMessage(msg)
	}
}

// buildExcludeSet builds a set from exceptIDs for O(1) lookups when the
// list is large enough to justify the allocation. Returns nil for small
// lists (≤4) — callers should use isExcludedByID which falls back to a
// linear scan.
func buildExcludeSet(exceptIDs []string) map[string]struct{} {
	if len(exceptIDs) <= 4 {
		return nil
	}
	exclude := make(map[string]struct{}, len(exceptIDs))
	for _, id := range exceptIDs {
		exclude[id] = struct{}{}
	}
	return exclude
}

// isExcludedByID reports whether clientID is in the exclusion list. When
// excludeSet is nil it falls back to a linear scan of exceptIDs.
func isExcludedByID(clientID string, exceptIDs []string, excludeSet map[string]struct{}) bool {
	if excludeSet != nil {
		_, skip := excludeSet[clientID]
		return skip
	}
	return slices.Contains(exceptIDs, clientID)
}

// buildClientExcludeSet builds a set from except for O(1) lookups when the
// list is large enough to justify the allocation. Returns nil for small
// lists (≤4) — callers should use isExcludedClient which falls back to a
// linear scan.
func buildClientExcludeSet(except []*Client) map[*Client]struct{} {
	if len(except) <= 4 {
		return nil
	}
	exclude := make(map[*Client]struct{}, len(except))
	for _, c := range except {
		exclude[c] = struct{}{}
	}
	return exclude
}

// isExcludedClient reports whether client is in the exclusion list. When
// excludeSet is nil it falls back to a linear scan of except.
func isExcludedClient(client *Client, except []*Client, excludeSet map[*Client]struct{}) bool {
	if excludeSet != nil {
		_, skip := excludeSet[client]
		return skip
	}
	return slices.Contains(except, client)
}

// broadcastExceptByIDs sends to all local clients whose IDs are not in the
// exclude list. Used when processing adapter messages where we only have IDs.
func (h *Hub) broadcastExceptByIDs(item sendItem, exceptIDs []string) {
	if len(exceptIDs) == 0 {
		h.broadcast(item)
		return
	}

	snap := h.loadSnapshot()
	excludeSet := buildExcludeSet(exceptIDs)
	for _, client := range snap.slice {
		if !isExcludedByID(client.ID, exceptIDs, excludeSet) {
			h.trySend(client, item)
		}
	}
}

// broadcastToRoomLocal sends to all local clients in a room without publishing
// to the adapter. Safe to call when the room does not exist locally.
// Respects the hub's parallel mode setting.
func (h *Hub) broadcastToRoomLocal(roomName string, item sendItem) {
	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return
	}

	h.broadcastToRoomClients(room, item)
}

// broadcastToRoomExceptByIDs sends to local room clients whose IDs are not
// in the exclude list.
func (h *Hub) broadcastToRoomExceptByIDs(roomName string, item sendItem, exceptIDs []string) {
	if len(exceptIDs) == 0 {
		h.broadcastToRoomLocal(roomName, item)
		return
	}

	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return
	}

	excludeSet := buildExcludeSet(exceptIDs)
	for _, client := range loadRoomSnapshot(room) {
		if !isExcludedByID(client.ID, exceptIDs, excludeSet) {
			h.trySend(client, item)
		}
	}
}

// sendToUserLocal sends to all local clients of a user without publishing
// to the adapter.
func (h *Hub) sendToUserLocal(userID string, item sendItem) {
	clients := h.GetClientsByUserID(userID)
	for _, client := range clients {
		h.trySend(client, item)
	}
}

// sendToClientLocal sends to a local client by ID without publishing to the
// adapter. Returns an error if the client is not found locally (which is
// expected in multi-node setups — the client may be on another node).
func (h *Hub) sendToClientLocal(clientID string, data []byte, msgType int) error {
	client, ok := h.GetClient(clientID)
	if !ok {
		return ErrClientNotFound
	}
	return client.SendMessage(MessageType(msgType), data)
}

// Clients returns all connected clients using the lock-free broadcast
// snapshot, avoiding contention on h.mu.
func (h *Hub) Clients() []*Client {
	snap := h.loadSnapshot()
	// Return a copy so callers cannot mutate the shared snapshot slice.
	out := make([]*Client, len(snap.slice))
	copy(out, snap.slice)
	return out
}

// ClientCount returns the number of connected clients.
// Uses an atomic counter — no locking required.
func (h *Hub) ClientCount() int {
	return int(h.clientCount.Load())
}

// GetClient returns a client by ID (O(1) lookup).
func (h *Hub) GetClient(id string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clientIndex[id]
	return client, ok
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

// parallelSend sends a sendItem to a slice of clients in parallel batches
// using a persistent worker pool. If the number of clients is at or below
// the batch size, it sends sequentially to avoid pool dispatch overhead.
func (h *Hub) parallelSend(clients []*Client, item sendItem) {
	if len(clients) == 0 {
		return
	}

	batchSize := h.parallelBatchSize
	if len(clients) <= batchSize {
		for _, client := range clients {
			h.trySend(client, item)
		}
		return
	}

	pool := h.ensurePool()
	numBatches := (len(clients) + batchSize - 1) / batchSize

	var wg sync.WaitGroup
	wg.Add(numBatches)

	for i := range numBatches {
		start := i * batchSize
		end := min(start+batchSize, len(clients))
		task := broadcastTask{
			hub:     h,
			clients: clients[start:end],
			item:    item,
			wg:      &wg,
		}
		if !pool.submit(task) {
			// Pool shut down — send remaining batches inline.
			remaining := numBatches - i
			for range remaining {
				wg.Done()
			}
			for _, client := range clients[start:] {
				h.trySend(client, item)
			}
			break
		}
	}

	wg.Wait()
}

// broadcast is the internal dispatch used by Broadcast and BroadcastBinary.
func (h *Hub) broadcast(item sendItem) {
	start := time.Now()
	snap := h.loadSnapshot()
	if h.useParallel {
		h.parallelSend(snap.slice, item)
	} else {
		for _, client := range snap.slice {
			h.trySend(client, item)
		}
	}
	h.metrics.RecordBroadcastDuration(time.Since(start))
}

// Broadcast sends a text message to all connected clients.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) Broadcast(data []byte) {
	h.broadcast(sendItem{msgType: websocket.TextMessage, data: data})
	h.publishToAdapter(AdapterMessage{
		Type:    AdapterBroadcast,
		MsgType: websocket.TextMessage,
		Data:    data,
	})
}

// BroadcastBinary sends a binary message to all connected clients.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastBinary(data []byte) {
	h.broadcast(sendItem{msgType: websocket.BinaryMessage, data: data})
	h.publishToAdapter(AdapterMessage{
		Type:    AdapterBroadcast,
		MsgType: websocket.BinaryMessage,
		Data:    data,
	})
}

// BroadcastWithContext sends a message to all clients with context support.
// If the context is cancelled mid-broadcast, remaining local clients are
// skipped but the adapter publish still fires so other nodes can deliver.
// The returned error (if any) is the context error.
func (h *Hub) BroadcastWithContext(ctx context.Context, data []byte) error {
	snap := h.loadSnapshot()
	item := sendItem{msgType: websocket.TextMessage, data: data}

	ctxErr := h.sendWithContext(ctx, snap.slice, item)

	// Always publish to the adapter so other nodes can deliver, even if the
	// local broadcast was cut short by context cancellation.
	h.publishToAdapter(AdapterMessage{
		Type:    AdapterBroadcast,
		MsgType: websocket.TextMessage,
		Data:    data,
	})
	return ctxErr
}

// sendWithContext sends to a list of clients with context cancellation support.
// When parallel mode is enabled, batches are dispatched to the worker pool
// and all stop early when the context is cancelled.
func (h *Hub) sendWithContext(ctx context.Context, clients []*Client, item sendItem) error {
	if len(clients) == 0 {
		return nil
	}

	if !h.useParallel || len(clients) <= h.parallelBatchSize {
		for _, client := range clients {
			if !h.trySendWithContext(ctx, client, item) {
				return ctx.Err()
			}
		}
		return nil
	}

	pool := h.ensurePool()
	batchSize := h.parallelBatchSize
	numBatches := (len(clients) + batchSize - 1) / batchSize

	var wg sync.WaitGroup
	wg.Add(numBatches)
	result := &contextResult{}

	for i := range numBatches {
		start := i * batchSize
		end := min(start+batchSize, len(clients))
		task := broadcastTask{
			hub:     h,
			clients: clients[start:end],
			item:    item,
			ctx:     ctx,
			result:  result,
			wg:      &wg,
		}
		if !pool.submit(task) {
			// Pool shut down — send remaining batches inline.
			remaining := numBatches - i
			for range remaining {
				wg.Done()
			}
			for _, client := range clients[start:] {
				if !h.trySendWithContext(ctx, client, item) {
					result.mu.Lock()
					if result.err == nil {
						result.err = ctx.Err()
					}
					result.mu.Unlock()
					break
				}
			}
			break
		}
	}

	wg.Wait()
	return result.err
}

// trySendWithContext sends to a client's channel with context cancellation.
// It recovers from panics caused by sending on a closed channel.
func (h *Hub) trySendWithContext(ctx context.Context, client *Client, item sendItem) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if isChanSendPanic(r) {
				ok = true // client disconnected, skip it — not a ctx error
				return
			}
			panic(r) // re-panic for unexpected errors
		}
	}()
	select {
	case client.send <- item:
		return true
	case <-ctx.Done():
		return false
	}
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

// BroadcastRawJSON sends pre-encoded JSON data as a text message to all
// connected clients. Use this instead of BroadcastJSON when the JSON is
// already marshaled to avoid redundant serialization.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastRawJSON(data []byte) {
	h.Broadcast(data)
}

// broadcastExceptClients sends to all clients in the slice not in the exclude list,
// using parallelSend when parallel mode is enabled. excludeSet may be nil for
// small lists — isExcludedClient handles the fallback to linear scan.
func (h *Hub) broadcastExceptClients(clients []*Client, item sendItem, except []*Client, excludeSet map[*Client]struct{}) {
	start := time.Now()
	if h.useParallel {
		filtered := make([]*Client, 0, len(clients))
		for _, client := range clients {
			if !isExcludedClient(client, except, excludeSet) {
				filtered = append(filtered, client)
			}
		}
		h.parallelSend(filtered, item)
	} else {
		for _, client := range clients {
			if !isExcludedClient(client, except, excludeSet) {
				h.trySend(client, item)
			}
		}
	}
	h.metrics.RecordBroadcastDuration(time.Since(start))
}

// BroadcastExcept sends a text message to all clients except those specified.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastExcept(data []byte, except ...*Client) {
	h.broadcastExceptWithType(data, websocket.TextMessage, except...)
}

// BroadcastBinaryExcept sends a binary message to all clients except those specified.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastBinaryExcept(data []byte, except ...*Client) {
	h.broadcastExceptWithType(data, websocket.BinaryMessage, except...)
}

func (h *Hub) broadcastExceptWithType(data []byte, msgType int, except ...*Client) {
	snap := h.loadSnapshot()
	item := sendItem{msgType: msgType, data: data}

	excludeSet := buildClientExcludeSet(except)
	h.broadcastExceptClients(snap.slice, item, except, excludeSet)

	if len(except) > 0 {
		ids := make([]string, len(except))
		for i, c := range except {
			ids[i] = c.ID
		}
		h.publishToAdapter(AdapterMessage{
			Type:            AdapterBroadcastExcept,
			MsgType:         msgType,
			Data:            data,
			ExceptClientIDs: ids,
		})
	} else {
		h.publishToAdapter(AdapterMessage{
			Type:    AdapterBroadcast,
			MsgType: msgType,
			Data:    data,
		})
	}
}

// SendToUser sends a text message to all clients of a specific user.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) SendToUser(userID string, data []byte) {
	h.sendToUserWithType(userID, data, websocket.TextMessage)
}

// SendBinaryToUser sends a binary message to all clients of a specific user.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) SendBinaryToUser(userID string, data []byte) {
	h.sendToUserWithType(userID, data, websocket.BinaryMessage)
}

func (h *Hub) sendToUserWithType(userID string, data []byte, msgType int) {
	clients := h.GetClientsByUserID(userID)
	item := sendItem{msgType: msgType, data: data}
	for _, client := range clients {
		h.trySend(client, item)
	}
	h.publishToAdapter(AdapterMessage{
		Type:    AdapterUser,
		MsgType: msgType,
		Data:    data,
		UserID:  userID,
	})
}

// SendToClient sends a text message to a specific client by ID.
// In multi-node mode the message is also relayed to other nodes via the adapter,
// allowing delivery to clients connected to a different node.
func (h *Hub) SendToClient(clientID string, data []byte) error {
	return h.sendToClientWithType(clientID, data, websocket.TextMessage)
}

// SendBinaryToClient sends a binary message to a specific client by ID.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) SendBinaryToClient(clientID string, data []byte) error {
	return h.sendToClientWithType(clientID, data, websocket.BinaryMessage)
}

func (h *Hub) sendToClientWithType(clientID string, data []byte, msgType int) error {
	client, ok := h.GetClient(clientID)
	if ok {
		return client.SendMessage(MessageType(msgType), data)
	}

	// Client not found locally — try via adapter.
	if h.adapter != nil {
		h.publishToAdapter(AdapterMessage{
			Type:     AdapterClient,
			MsgType:  msgType,
			Data:     data,
			ClientID: clientID,
		})
		return nil
	}

	return ErrClientNotFound
}

// SendToUserWithContext sends a text message to all clients of a specific user
// with context support. It blocks until messages are enqueued or the context
// is cancelled. In multi-node mode the message is also relayed via the adapter.
func (h *Hub) SendToUserWithContext(ctx context.Context, userID string, data []byte) error {
	clients := h.GetClientsByUserID(userID)
	item := sendItem{msgType: websocket.TextMessage, data: data}
	ctxErr := h.sendWithContext(ctx, clients, item)

	h.publishToAdapter(AdapterMessage{
		Type:    AdapterUser,
		MsgType: websocket.TextMessage,
		Data:    data,
		UserID:  userID,
	})
	return ctxErr
}

// SendToClientWithContext sends a text message to a specific client by ID
// with context support. It blocks until the message is enqueued or the context
// is cancelled. In multi-node mode the message is also relayed via the adapter.
func (h *Hub) SendToClientWithContext(ctx context.Context, clientID string, data []byte) error {
	client, ok := h.GetClient(clientID)
	if ok {
		return client.SendMessageWithContext(ctx, TextMessage, data)
	}

	if h.adapter != nil {
		h.publishToAdapter(AdapterMessage{
			Type:     AdapterClient,
			MsgType:  websocket.TextMessage,
			Data:     data,
			ClientID: clientID,
		})
		return nil
	}

	return ErrClientNotFound
}

// JoinRoom adds a client to a room.
func (h *Hub) JoinRoom(client *Client, roomName string) error {
	if roomName == "" {
		return ErrEmptyRoomName
	}

	// Check if client exists
	h.mu.RLock()
	if _, ok := h.clients[client]; !ok {
		h.mu.RUnlock()
		return ErrClientNotFound
	}
	h.mu.RUnlock()

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
			clients: make(map[*Client]struct{}),
		}
		h.rooms[roomName] = room
		h.metrics.IncrementRooms()
	}
	h.roomsMu.Unlock()

	// Lock the room and the client atomically to check both limits
	// in a single critical section, eliminating the TOCTOU race.
	room.mu.Lock()

	// cleanupRoom removes a newly created room if a limit check fails,
	// preventing empty rooms from leaking into h.rooms.
	//
	// The unlock/relock dance (room → roomsMu → room) creates a window
	// where another goroutine could add clients to the room. The second
	// len(room.clients)==0 check after re-acquiring both locks ensures we
	// never delete a room that gained members during that window.
	cleanupRoom := func() {
		if !exists && len(room.clients) == 0 {
			room.mu.Unlock()
			h.deleteRoomIfEmpty(roomName, room)
		} else {
			room.mu.Unlock()
		}
	}

	// Check room size limit
	if h.limits.MaxClientsPerRoom > 0 && len(room.clients) >= h.limits.MaxClientsPerRoom {
		cleanupRoom()
		return fmt.Errorf("room %q: %w", roomName, ErrRoomFull)
	}

	// Check if already in room
	if _, ok := room.clients[client]; ok {
		cleanupRoom()
		return ErrAlreadyInRoom
	}

	// Check per-client room limit under room lock + client lock to
	// prevent two concurrent JoinRoom calls both passing the check.
	client.mu.Lock()
	if h.limits.MaxRoomsPerClient > 0 && len(client.rooms) >= h.limits.MaxRoomsPerClient {
		client.mu.Unlock()
		cleanupRoom()
		return fmt.Errorf("client %s: %w", client.ID, ErrMaxRoomsReached)
	}
	client.rooms[roomName] = struct{}{}
	client.mu.Unlock()

	room.clients[client] = struct{}{}
	rebuildRoomSnapshot(room)
	roomSize := len(room.clients)
	room.mu.Unlock()
	h.roomVersion.Add(1)

	h.metrics.IncrementRoomJoins()
	h.logger.Debug("Client joined room",
		"clientID", client.ID,
		"room", roomName,
		"roomSize", roomSize,
	)

	// Call AfterRoomJoin hook
	if h.hooks.AfterRoomJoin != nil {
		go h.hooks.AfterRoomJoin(client, roomName)
	}

	return nil
}

// LeaveRoom removes a client from a room.
func (h *Hub) LeaveRoom(client *Client, roomName string) error {
	if roomName == "" {
		return ErrEmptyRoomName
	}

	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if !ok {
		return ErrRoomNotFound
	}

	// Check membership before calling the hook so the hook runs
	// without room.mu held, preventing deadlocks when the hook
	// queries the same room (e.g. RoomCount, RoomClients).
	room.mu.RLock()
	_, inRoom := room.clients[client]
	room.mu.RUnlock()
	if !inRoom {
		return ErrNotInRoom
	}

	if h.hooks.BeforeRoomLeave != nil {
		h.hooks.BeforeRoomLeave(client, roomName)
	}

	// Re-check under write lock — client may have left concurrently.
	room.mu.Lock()
	if _, inRoom := room.clients[client]; !inRoom {
		room.mu.Unlock()
		return ErrNotInRoom
	}
	delete(room.clients, client)
	rebuildRoomSnapshot(room)
	client.leaveRoom(roomName)
	h.roomVersion.Add(1)
	roomEmpty := len(room.clients) == 0
	room.mu.Unlock()

	if roomEmpty {
		h.deleteRoomIfEmpty(roomName, room)
	}

	h.metrics.IncrementRoomLeaves()
	h.logger.Debug("Client left room",
		"clientID", client.ID,
		"room", roomName,
	)

	if h.hooks.AfterRoomLeave != nil {
		go h.hooks.AfterRoomLeave(client, roomName)
	}

	return nil
}

// LeaveAllRooms removes a client from all rooms, firing
// BeforeRoomLeave/AfterRoomLeave hooks for each room.
func (h *Hub) LeaveAllRooms(client *Client) {
	h.removeClientFromAllRoomsWithHooks(client)
}

// broadcastToRoomClients sends a sendItem to all clients in a room.
// It snapshots membership under the room lock, then releases before
// sending to avoid holding the lock during potentially slow channel ops.
func (h *Hub) broadcastToRoomClients(room *Room, item sendItem) {
	start := time.Now()
	clients := loadRoomSnapshot(room)

	if h.useParallel {
		h.parallelSend(clients, item)
	} else {
		for _, client := range clients {
			h.trySend(client, item)
		}
	}
	h.metrics.RecordBroadcastDuration(time.Since(start))
}

// BroadcastToRoom sends a text message to all clients in a room.
// In multi-node mode the message is also relayed to other nodes via the adapter
// so that room members on other nodes receive it.
func (h *Hub) BroadcastToRoom(roomName string, data []byte) error {
	return h.broadcastToRoomWithType(roomName, data, websocket.TextMessage)
}

// BroadcastBinaryToRoom sends a binary message to all clients in a room.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastBinaryToRoom(roomName string, data []byte) error {
	return h.broadcastToRoomWithType(roomName, data, websocket.BinaryMessage)
}

func (h *Hub) broadcastToRoomWithType(roomName string, data []byte, msgType int) error {
	if roomName == "" {
		return ErrEmptyRoomName
	}

	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if ok {
		h.broadcastToRoomClients(room, sendItem{msgType: msgType, data: data})
	}

	// Publish to adapter regardless of local room existence — the room
	// may have members on other nodes.
	h.publishToAdapter(AdapterMessage{
		Type:    AdapterRoom,
		MsgType: msgType,
		Data:    data,
		Room:    roomName,
	})

	if !ok && h.adapter == nil {
		return ErrRoomNotFound
	}
	return nil
}

// BroadcastToRoomWithContext sends a text message to all clients in a room with
// context support. Returns ctx.Err() if the context is cancelled mid-broadcast.
func (h *Hub) BroadcastToRoomWithContext(ctx context.Context, roomName string, data []byte) error {
	if roomName == "" {
		return ErrEmptyRoomName
	}

	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	var ctxErr error
	if ok {
		item := sendItem{msgType: websocket.TextMessage, data: data}
		clients := loadRoomSnapshot(room)
		ctxErr = h.sendWithContext(ctx, clients, item)
	}

	// Always publish to the adapter so other nodes can deliver, even if the
	// local broadcast was cut short by context cancellation.
	h.publishToAdapter(AdapterMessage{
		Type:    AdapterRoom,
		MsgType: websocket.TextMessage,
		Data:    data,
		Room:    roomName,
	})

	if ctxErr != nil {
		return ctxErr
	}
	if !ok && h.adapter == nil {
		return ErrRoomNotFound
	}
	return nil
}

// BroadcastToRoomExcept sends a text message to all clients in a room except those specified.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastToRoomExcept(roomName string, data []byte, except ...*Client) error {
	return h.broadcastToRoomExceptWithType(roomName, data, websocket.TextMessage, except...)
}

// BroadcastBinaryToRoomExcept sends a binary message to all clients in a room except those specified.
// In multi-node mode the message is also relayed to other nodes via the adapter.
func (h *Hub) BroadcastBinaryToRoomExcept(roomName string, data []byte, except ...*Client) error {
	return h.broadcastToRoomExceptWithType(roomName, data, websocket.BinaryMessage, except...)
}

func (h *Hub) broadcastToRoomExceptWithType(roomName string, data []byte, msgType int, except ...*Client) error {
	if roomName == "" {
		return ErrEmptyRoomName
	}

	h.roomsMu.RLock()
	room, ok := h.rooms[roomName]
	h.roomsMu.RUnlock()

	if ok {
		snapshot := loadRoomSnapshot(room)
		item := sendItem{msgType: msgType, data: data}
		h.broadcastExceptClients(snapshot, item, except, buildClientExcludeSet(except))
	}

	// Publish to adapter — room may have members on other nodes.
	// Use AdapterRoom (simpler) when there are no exclusions.
	if len(except) > 0 {
		exceptIDs := make([]string, len(except))
		for i, c := range except {
			exceptIDs[i] = c.ID
		}
		h.publishToAdapter(AdapterMessage{
			Type:            AdapterRoomExcept,
			MsgType:         msgType,
			Data:            data,
			Room:            roomName,
			ExceptClientIDs: exceptIDs,
		})
	} else {
		h.publishToAdapter(AdapterMessage{
			Type:    AdapterRoom,
			MsgType: msgType,
			Data:    data,
			Room:    roomName,
		})
	}

	if !ok && h.adapter == nil {
		return ErrRoomNotFound
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

	snapshot := loadRoomSnapshot(room)
	// Return a copy so callers cannot mutate the shared snapshot.
	clients := make([]*Client, len(snapshot))
	copy(clients, snapshot)
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

	return len(loadRoomSnapshot(room))
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
