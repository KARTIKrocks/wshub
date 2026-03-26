package wshub

import (
	"encoding/json"
	"time"
)

// nodePresence is the wire format published by each hub on every presence tick.
type nodePresence struct {
	NodeID      string         `json:"node_id"`
	ClientCount int            `json:"client_count"`
	Rooms       map[string]int `json:"rooms"`
	Timestamp   int64          `json:"ts"`
}

// nodeStats is the cached representation of a remote node's last presence report.
type nodeStats struct {
	clientCount int
	rooms       map[string]int
	lastSeen    time.Time
}

// presenceState holds cached state between presence ticks to avoid
// re-gathering room stats when unchanged.
type presenceState struct {
	lastClientCount int
	lastRoomVersion int64
	cachedPayload   []byte
	lastPresence    *nodePresence
}

// runPresence periodically publishes local stats and evicts stale nodes.
// It is started as a goroutine from Run() when presence is enabled.
func (h *Hub) runPresence() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.presenceInterval)
	defer ticker.Stop()

	// Uses the atomic roomVersion counter (O(1)) instead of maps.Equal
	// (O(rooms)) to detect room changes.
	// Initialize lastRoomVersion to -1 so the first tick always gathers
	// stats, even when the hub has zero clients and roomVersion is 0.
	state := presenceState{lastRoomVersion: -1}

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.publishPresence(&state)
			h.evictStaleNodes()
		}
	}
}

// publishPresence gathers local stats and publishes them to the adapter.
// It caches the room/client stats and only re-gathers when the stats have
// changed, but always marshals a fresh timestamp so remote nodes see an
// accurate heartbeat time.
func (h *Hub) publishPresence(s *presenceState) {
	clientCount := h.ClientCount()
	roomVersion := h.roomVersion.Load()

	// Re-gather room stats only when they have changed.
	if clientCount != s.lastClientCount || roomVersion != s.lastRoomVersion {
		s.lastClientCount = clientCount
		s.lastRoomVersion = roomVersion

		// Gather local room counts using lock-free snapshots.
		h.roomsMu.RLock()
		rooms := make(map[string]int, len(h.rooms))
		for name, room := range h.rooms {
			rooms[name] = len(loadRoomSnapshot(room))
		}
		h.roomsMu.RUnlock()

		p := &nodePresence{
			NodeID:      h.nodeID,
			ClientCount: clientCount,
			Rooms:       rooms,
			Timestamp:   time.Now().UnixMilli(),
		}
		s.lastPresence = p

		data, err := json.Marshal(p)
		if err != nil {
			h.logger.Error("presence marshal failed", "error", err)
			return
		}
		s.cachedPayload = data
	} else if s.lastPresence != nil {
		// Stats unchanged — update only the timestamp so remote nodes see
		// a fresh heartbeat. presenceState is only accessed from the single
		// runPresence goroutine, so in-place mutation is safe.
		s.lastPresence.Timestamp = time.Now().UnixMilli()

		data, err := json.Marshal(s.lastPresence)
		if err != nil {
			h.logger.Error("presence marshal failed", "error", err)
			return
		}
		s.cachedPayload = data
	}

	if len(s.cachedPayload) == 0 {
		return
	}

	// Always publish to keep the heartbeat alive.
	h.publishToAdapter(AdapterMessage{
		Type: AdapterPresence,
		Data: s.cachedPayload,
	})
}

// handlePresenceMessage processes a presence report from a remote node.
func (h *Hub) handlePresenceMessage(msg AdapterMessage) {
	// Only cache if presence is enabled on this hub.
	if h.presenceCache == nil {
		return
	}

	var p nodePresence
	if err := json.Unmarshal(msg.Data, &p); err != nil {
		h.logger.Warn("presence unmarshal failed", "error", err)
		return
	}

	h.presenceMu.Lock()
	h.presenceCache[p.NodeID] = &nodeStats{
		clientCount: p.ClientCount,
		rooms:       p.Rooms,
		lastSeen:    time.Now(),
	}
	h.presenceMu.Unlock()
}

// evictStaleNodes removes cached stats for nodes that have not reported
// within the TTL window.
func (h *Hub) evictStaleNodes() {
	cutoff := time.Now().Add(-h.presenceTTL)

	h.presenceMu.Lock()
	for nodeID, stats := range h.presenceCache {
		if stats.lastSeen.Before(cutoff) {
			delete(h.presenceCache, nodeID)
			h.logger.Info("evicted stale node", "nodeID", nodeID)
		}
	}
	h.presenceMu.Unlock()
}

// GlobalClientCount returns the total number of connected clients across all
// nodes. In single-node mode or when presence is not enabled it returns the
// local count.
func (h *Hub) GlobalClientCount() int {
	total := h.ClientCount()

	if h.presenceCache == nil {
		return total
	}

	h.presenceMu.RLock()
	for _, stats := range h.presenceCache {
		total += stats.clientCount
	}
	h.presenceMu.RUnlock()

	return total
}

// GlobalRoomCount returns the total number of clients in a room across all
// nodes. In single-node mode or when presence is not enabled it returns the
// local room count.
func (h *Hub) GlobalRoomCount(roomName string) int {
	total := h.RoomCount(roomName)

	if h.presenceCache == nil {
		return total
	}

	h.presenceMu.RLock()
	for _, stats := range h.presenceCache {
		if stats.rooms != nil {
			total += stats.rooms[roomName]
		}
	}
	h.presenceMu.RUnlock()

	return total
}
