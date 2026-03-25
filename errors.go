package wshub

import "errors"

// Sentinel errors for WebSocket operations.
var (
	// Connection errors
	ErrConnectionClosed = errors.New("wshub: connection closed")
	ErrWriteTimeout     = errors.New("wshub: write timeout")
	ErrReadTimeout      = errors.New("wshub: read timeout")
	ErrInvalidMessage   = errors.New("wshub: invalid message")

	// Client errors
	ErrClientNotFound      = errors.New("wshub: client not found")
	ErrClientAlreadyExists = errors.New("wshub: client already exists")

	// Room errors
	ErrEmptyRoomName = errors.New("wshub: empty room name")
	ErrRoomNotFound  = errors.New("wshub: room not found")
	ErrAlreadyInRoom = errors.New("wshub: client already in room")
	ErrNotInRoom     = errors.New("wshub: client not in room")
	ErrRoomFull      = errors.New("wshub: room is full")

	// Limit errors
	ErrMaxConnectionsReached     = errors.New("wshub: max connections reached")
	ErrMaxRoomsReached           = errors.New("wshub: max rooms per client reached")
	ErrRateLimitExceeded         = errors.New("wshub: rate limit exceeded")
	ErrMaxUserConnectionsReached = errors.New("wshub: max connections per user reached")

	// Authentication errors
	ErrAuthenticationFailed = errors.New("wshub: authentication failed")
	ErrUnauthorized         = errors.New("wshub: unauthorized")
)
