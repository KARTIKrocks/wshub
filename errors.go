package websocket

import "errors"

// Sentinel errors for WebSocket operations.
var (
	// Connection errors
	ErrConnectionClosed = errors.New("websocket: connection closed")
	ErrWriteTimeout     = errors.New("websocket: write timeout")
	ErrReadTimeout      = errors.New("websocket: read timeout")
	ErrInvalidMessage   = errors.New("websocket: invalid message")

	// Client errors
	ErrClientNotFound      = errors.New("websocket: client not found")
	ErrClientAlreadyExists = errors.New("websocket: client already exists")

	// Room errors
	ErrRoomNotFound  = errors.New("websocket: room not found")
	ErrAlreadyInRoom = errors.New("websocket: client already in room")
	ErrNotInRoom     = errors.New("websocket: client not in room")
	ErrRoomFull      = errors.New("websocket: room is full")

	// Limit errors
	ErrMaxConnectionsReached = errors.New("websocket: max connections reached")
	ErrMaxRoomsReached       = errors.New("websocket: max rooms per client reached")
	ErrRateLimitExceeded     = errors.New("websocket: rate limit exceeded")

	// Authentication errors
	ErrAuthenticationFailed = errors.New("websocket: authentication failed")
	ErrUnauthorized         = errors.New("websocket: unauthorized")

	// Codec errors
	ErrEncodingFailed = errors.New("websocket: encoding failed")
	ErrDecodingFailed = errors.New("websocket: decoding failed")
)
