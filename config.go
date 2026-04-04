package wshub

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType represents the type of WebSocket message.
type MessageType int

const (
	TextMessage   MessageType = websocket.TextMessage
	BinaryMessage MessageType = websocket.BinaryMessage
	CloseMessage  MessageType = websocket.CloseMessage
	PingMessage   MessageType = websocket.PingMessage
	PongMessage   MessageType = websocket.PongMessage
)

// Config holds configuration for WebSocket connections.
type Config struct {
	// ReadBufferSize is the size of the read buffer (default: 1024).
	ReadBufferSize int

	// WriteBufferSize is the size of the write buffer (default: 1024).
	WriteBufferSize int

	// WriteWait is the time allowed to write a message (default: 10s).
	WriteWait time.Duration

	// PongWait is the time allowed to read the next pong message (default: 60s).
	PongWait time.Duration

	// PingPeriod is the period between pings (default: 54s, must be < PongWait).
	PingPeriod time.Duration

	// MaxMessageSize is the maximum message size allowed (default: 512KB).
	MaxMessageSize int64

	// SendChannelSize is the size of the send channel buffer (default: 256).
	SendChannelSize int

	// EnableCompression enables per-message compression (default: false).
	EnableCompression bool

	// CoalesceWrites batches queued text messages into a single WebSocket
	// frame separated by newline bytes (\n), reducing syscalls under high
	// throughput. Binary messages are always sent as individual frames.
	// Receivers must split coalesced frames on \n. Default: false.
	CoalesceWrites bool

	// CheckOrigin is a function to validate the request origin.
	CheckOrigin func(r *http.Request) bool

	// Subprotocols specifies the server's supported protocols.
	Subprotocols []string
}

// DefaultConfig returns a default WebSocket configuration.
func DefaultConfig() Config {
	return Config{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		WriteWait:         10 * time.Second,
		PongWait:          60 * time.Second,
		PingPeriod:        54 * time.Second,
		MaxMessageSize:    512 * 1024, // 512KB
		SendChannelSize:   256,
		EnableCompression: false,
		CheckOrigin:       AllowAllOrigins,
	}
}

// applyConfigDefaults fills zero-value fields in c with defaults from
// DefaultConfig so that Config{ReadBufferSize: 4096} behaves identically
// to DefaultConfig().WithBufferSizes(4096, 1024) for every unset field.
// It also corrects invalid relationships (e.g. PingPeriod >= PongWait).
func applyConfigDefaults(c Config) Config {
	d := DefaultConfig()
	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = d.ReadBufferSize
	}
	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = d.WriteBufferSize
	}
	if c.WriteWait <= 0 {
		c.WriteWait = d.WriteWait
	}
	if c.PongWait <= 0 {
		c.PongWait = d.PongWait
	}
	if c.PingPeriod <= 0 {
		c.PingPeriod = d.PingPeriod
	}
	if c.MaxMessageSize <= 0 {
		c.MaxMessageSize = d.MaxMessageSize
	}
	if c.SendChannelSize <= 0 {
		c.SendChannelSize = d.SendChannelSize
	}
	if c.CheckOrigin == nil {
		c.CheckOrigin = d.CheckOrigin
	}

	// PingPeriod must be less than PongWait to avoid premature timeouts.
	if c.PingPeriod >= c.PongWait {
		c.PingPeriod = (c.PongWait * 9) / 10 // 90% of PongWait
	}

	return c
}

// validateConfig checks for configuration invariants that applyConfigDefaults
// cannot silently fix. It returns a slice of human-readable warnings.
func validateConfig(c Config) []string {
	var warnings []string
	if c.ReadBufferSize > 0 && c.ReadBufferSize < 128 {
		warnings = append(warnings, fmt.Sprintf("ReadBufferSize (%d) is very small (< 128 bytes)", c.ReadBufferSize))
	}
	if c.WriteBufferSize > 0 && c.WriteBufferSize < 128 {
		warnings = append(warnings, fmt.Sprintf("WriteBufferSize (%d) is very small (< 128 bytes)", c.WriteBufferSize))
	}
	return warnings
}

// WithBufferSizes returns a new config with the specified buffer sizes.
func (c Config) WithBufferSizes(read, write int) Config {
	c.ReadBufferSize = read
	c.WriteBufferSize = write
	return c
}

// WithTimeouts returns a new config with the specified timeouts.
func (c Config) WithTimeouts(writeWait, pongWait, pingPeriod time.Duration) Config {
	c.WriteWait = writeWait
	c.PongWait = pongWait
	c.PingPeriod = pingPeriod
	return c
}

// WithMaxMessageSize returns a new config with the specified max message size.
func (c Config) WithMaxMessageSize(size int64) Config {
	c.MaxMessageSize = size
	return c
}

// WithSendChannelSize returns a new config with the specified send channel size.
func (c Config) WithSendChannelSize(size int) Config {
	c.SendChannelSize = size
	return c
}

// WithCompression returns a new config with compression enabled/disabled.
func (c Config) WithCompression(enabled bool) Config {
	c.EnableCompression = enabled
	return c
}

// WithCoalesceWrites returns a new config with write coalescing enabled/disabled.
func (c Config) WithCoalesceWrites(enabled bool) Config {
	c.CoalesceWrites = enabled
	return c
}

// WithCheckOrigin returns a new config with a custom origin checker.
func (c Config) WithCheckOrigin(fn func(r *http.Request) bool) Config {
	c.CheckOrigin = fn
	return c
}

// WithSubprotocols returns a new config with the specified subprotocols.
func (c Config) WithSubprotocols(protocols ...string) Config {
	c.Subprotocols = protocols
	return c
}

// AllowAllOrigins is a CheckOrigin function that allows all origins.
func AllowAllOrigins(r *http.Request) bool {
	return true
}

// AllowSameOrigin is a CheckOrigin function that only allows same-origin requests.
// It parses the Origin header as a URL and compares the host (including port)
// against the request's Host header, handling mismatched ports correctly.
//
// Requests without an Origin header are allowed because non-browser clients
// (mobile apps, CLI tools) typically omit it. If your threat model requires
// rejecting originless requests, use a custom CheckOrigin function.
func AllowSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

// AllowOrigins returns a CheckOrigin function that allows specific origins.
// Requests without an Origin header are allowed (see AllowSameOrigin for rationale).
func AllowOrigins(origins ...string) func(r *http.Request) bool {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[o] = struct{}{}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}
}
