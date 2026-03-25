package wshub

import "context"

// Adapter enables multi-node Hub communication through a shared message bus.
// When set via WithAdapter, every public broadcast and send method publishes
// to the adapter after delivering locally, so that other nodes can relay
// the message to their own clients.
//
// Thread safety requirements:
//   - Publish must be safe for concurrent calls from multiple goroutines.
//   - Subscribe is called once by Hub.Run and must not be called concurrently.
//   - Publish and Subscribe may be called concurrently with each other.
//   - Close may be called concurrently with Publish; implementations should
//     handle this gracefully (e.g., return an error after close).
type Adapter interface {
	// Publish sends a message to all other nodes.
	Publish(ctx context.Context, msg AdapterMessage) error

	// Subscribe begins receiving messages from other nodes. The provided
	// handler is invoked for every message received. Subscribe must not
	// block; it should spawn its own goroutine(s) internally.
	// The context controls the subscription lifetime — cancelling it
	// stops the subscriber.
	Subscribe(ctx context.Context, handler func(AdapterMessage)) error

	// Close shuts down the adapter, releasing all resources.
	Close() error
}

// Adapter message type constants identify the operation being relayed.
const (
	AdapterBroadcast       = "broadcast"
	AdapterBroadcastExcept = "broadcast_except"
	AdapterRoom            = "room"
	AdapterRoomExcept      = "room_except"
	AdapterUser            = "user"
	AdapterClient          = "client"
	AdapterPresence        = "presence"
)

// AdapterMessage is the wire format for inter-node messages.
type AdapterMessage struct {
	// NodeID identifies the originating hub node (used for deduplication).
	NodeID string `json:"node_id"`

	// Type identifies the operation. Use the Adapter* constants.
	Type string `json:"type"`

	// Room is the target room name (for room-scoped operations).
	Room string `json:"room,omitempty"`

	// UserID is the target user (for SendToUser).
	UserID string `json:"user_id,omitempty"`

	// ClientID is the target client (for SendToClient).
	ClientID string `json:"client_id,omitempty"`

	// ExceptClientIDs lists client IDs to exclude from delivery.
	ExceptClientIDs []string `json:"except,omitempty"`

	// MsgType is the WebSocket message type (TextMessage or BinaryMessage).
	MsgType int `json:"msg_type"`

	// Data is the raw message payload.
	Data []byte `json:"data"`
}
