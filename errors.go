package wshub

import (
	"errors"
	"strings"
)

// isChanSendPanic reports whether a recovered panic value indicates a
// "send on closed channel" runtime error. This is the only panic that
// can originate from a channel send operation in Go.
//
// Centralised here so the fragile string match lives in one place.
func isChanSendPanic(r any) bool {
	if r == nil {
		return false
	}
	var msg string
	switch v := r.(type) {
	case error:
		msg = v.Error()
	case string:
		msg = v
	default:
		return false
	}
	return strings.Contains(msg, "send on closed channel")
}

// Sentinel errors for WebSocket operations.
var (
	// Connection errors
	ErrConnectionClosed = errors.New("wshub: connection closed")
	ErrInvalidMessage   = errors.New("wshub: invalid message")
	ErrSendBufferFull   = errors.New("wshub: send buffer full")

	// Client errors
	ErrClientNotFound = errors.New("wshub: client not found")

	// Room errors
	ErrEmptyRoomName = errors.New("wshub: empty room name")
	ErrRoomNotFound  = errors.New("wshub: room not found")
	ErrAlreadyInRoom = errors.New("wshub: client already in room")
	ErrNotInRoom     = errors.New("wshub: client not in room")
	ErrRoomFull      = errors.New("wshub: room is full")

	// Limit errors
	ErrMaxConnectionsReached = errors.New("wshub: max connections reached")
	ErrMaxRoomsReached       = errors.New("wshub: max rooms per client reached")
	// ErrRateLimitExceeded is provided for use in application hooks and
	// handlers. The library's internal rate limiter drops messages silently
	// without returning this error.
	ErrRateLimitExceeded         = errors.New("wshub: rate limit exceeded")
	ErrMaxUserConnectionsReached = errors.New("wshub: max connections per user reached")

	// Authentication errors — provided for use in BeforeConnect hooks and
	// application-level handlers. The library does not return these directly.
	ErrAuthenticationFailed = errors.New("wshub: authentication failed")
	ErrUnauthorized         = errors.New("wshub: unauthorized")
)
