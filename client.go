package wshub

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// sendItem carries a message type and data through the send channel.
type sendItem struct {
	msgType int
	data    []byte
}

// Client represents a WebSocket client connection.
type Client struct {
	// ID is the unique identifier for this client.
	ID string

	hub    *Hub
	conn   *websocket.Conn
	send   chan sendItem
	config Config

	// Unexported fields protected by mu
	mu          sync.RWMutex
	userID      string
	metadata    map[string]any
	rooms       map[string]struct{}
	closed      bool
	connectedAt time.Time
	closedAt    time.Time
	request     *http.Request

	// Rate limiting
	msgCount      int64
	msgWindowStart atomic.Value // time.Time

	// Callbacks protected by callbackMu
	callbackMu sync.RWMutex
	onMessage  func(*Client, *Message)
	onClose    func(*Client)
	onError    func(*Client, error)
}

// newClient creates a new client from a WebSocket connection.
func newClient(hub *Hub, conn *websocket.Conn, config Config, r *http.Request) *Client {
	c := &Client{
		ID:          uuid.New().String(),
		hub:         hub,
		conn:        conn,
		send:        make(chan sendItem, config.SendChannelSize),
		config:      config,
		metadata:    make(map[string]any),
		rooms:       make(map[string]struct{}),
		connectedAt: time.Now(),
		request:     r,
	}
	c.msgWindowStart.Store(time.Now())
	return c
}

// Request returns the HTTP request that initiated this WebSocket connection.
// Use it to access headers, query params, remote address, and other request data.
func (c *Client) Request() *http.Request {
	return c.request
}

// ConnectedAt returns the time when this client connected.
func (c *Client) ConnectedAt() time.Time {
	return c.connectedAt
}

// SetUserID sets the user ID for authenticated clients.
// Returns an error if MaxConnectionsPerUser limit would be exceeded.
func (c *Client) SetUserID(userID string) error {
	c.mu.Lock()
	oldUserID := c.userID
	c.mu.Unlock()

	if oldUserID == userID {
		return nil
	}

	// Check per-user connection limit before allowing the change
	if !c.hub.canAcceptUserConnection(userID) {
		return ErrMaxUserConnectionsReached
	}

	c.mu.Lock()
	c.userID = userID
	c.mu.Unlock()

	// Update the hub's user index
	c.hub.UpdateClientUserID(c, oldUserID, userID)
	return nil
}

// GetUserID returns the user ID.
func (c *Client) GetUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

// SetMetadata sets a metadata value.
func (c *Client) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata returns a metadata value.
func (c *Client) GetMetadata(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.metadata[key]
	return v, ok
}

// DeleteMetadata removes a metadata value.
func (c *Client) DeleteMetadata(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.metadata, key)
}

// OnMessage sets the message handler for this client.
func (c *Client) OnMessage(fn func(*Client, *Message)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.onMessage = fn
}

// OnClose sets the close handler for this client.
func (c *Client) OnClose(fn func(*Client)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.onClose = fn
}

// OnError sets the error handler for this client.
func (c *Client) OnError(fn func(*Client, error)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.onError = fn
}

// Send sends a text message to the client.
func (c *Client) Send(data []byte) error {
	return c.SendMessage(TextMessage, data)
}

// SendText sends a text message to the client.
func (c *Client) SendText(text string) error {
	return c.Send([]byte(text))
}

// SendJSON sends a JSON-encoded message to the client.
func (c *Client) SendJSON(v any) error {
	msg, err := NewJSONMessage(v)
	if err != nil {
		return err
	}
	return c.Send(msg.Data)
}

// SendBinary sends a binary message to the client.
func (c *Client) SendBinary(data []byte) error {
	return c.SendMessage(BinaryMessage, data)
}

// SendMessage sends a message with the specified type.
func (c *Client) SendMessage(msgType MessageType, data []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	item := sendItem{msgType: int(msgType), data: data}
	select {
	case c.send <- item:
		return nil
	default:
		c.hub.logger.Warn("Send buffer full, message dropped",
			"clientID", c.ID,
			"bufferSize", len(c.send),
		)
		c.hub.metrics.IncrementErrors("send_buffer_full")
		return ErrWriteTimeout
	}
}

// SendWithContext sends a message with context support.
func (c *Client) SendWithContext(ctx context.Context, data []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	item := sendItem{msgType: websocket.TextMessage, data: data}
	select {
	case c.send <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrWriteTimeout
	}
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.CloseWithCode(websocket.CloseNormalClosure, "")
}

// CloseWithCode closes the client connection with a specific close code and reason.
func (c *Client) CloseWithCode(code int, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.closedAt = time.Now()
	c.mu.Unlock()

	// Send close message
	message := websocket.FormatCloseMessage(code, reason)
	c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(c.config.WriteWait))

	close(c.send)
	return c.conn.Close()
}

// IsClosed returns true if the connection is closed.
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// ClosedAt returns the time when the connection was closed.
func (c *Client) ClosedAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closedAt
}

// Rooms returns the list of rooms the client is in.
func (c *Client) Rooms() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rooms := make([]string, 0, len(c.rooms))
	for room := range c.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// InRoom checks if the client is in a specific room.
func (c *Client) InRoom(room string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.rooms[room]
	return ok
}

// RoomCount returns the number of rooms the client is in.
func (c *Client) RoomCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.rooms)
}

// joinRoom marks the client as joined to a room.
func (c *Client) joinRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[room] = struct{}{}
}

// leaveRoom marks the client as left from a room.
func (c *Client) leaveRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, room)
}

// checkRateLimit checks if the client has exceeded the message rate limit.
// Returns true if the message should be allowed.
func (c *Client) checkRateLimit() bool {
	maxRate := c.hub.limits.MaxMessageRate
	if maxRate <= 0 {
		return true
	}

	now := time.Now()
	windowStart := c.msgWindowStart.Load().(time.Time)

	// Reset window if more than 1 second has passed
	if now.Sub(windowStart) >= time.Second {
		c.msgWindowStart.Store(now)
		atomic.StoreInt64(&c.msgCount, 1)
		return true
	}

	count := atomic.AddInt64(&c.msgCount, 1)
	return count <= int64(maxRate)
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// Use a goroutine watching ctx.Done() to close the connection and unblock ReadMessage.
	go func() {
		<-ctx.Done()
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
				websocket.CloseNormalClosure) {

				c.callbackMu.RLock()
				errorHandler := c.onError
				c.callbackMu.RUnlock()

				if errorHandler != nil {
					errorHandler(c, err)
				}

				// Call hub-level error hook
				if c.hub.hooks.OnError != nil {
					c.hub.hooks.OnError(c, err)
				}

				c.hub.metrics.IncrementErrors("read_error")
			}
			return
		}

		// Record metrics
		c.hub.metrics.IncrementMessages()
		c.hub.metrics.RecordMessageSize(len(data))

		// Check rate limit
		if !c.checkRateLimit() {
			c.hub.logger.Warn("Rate limit exceeded, dropping message",
				"clientID", c.ID,
			)
			c.hub.metrics.IncrementErrors("rate_limit_exceeded")
			continue
		}

		msg := &Message{
			Type:     MessageType(messageType),
			Data:     data,
			ClientID: c.ID,
			Time:     time.Now(),
		}

		// Call BeforeMessage hook
		if c.hub.hooks.BeforeMessage != nil {
			modifiedMsg, err := c.hub.hooks.BeforeMessage(c, msg)
			if err != nil {
				c.hub.logger.Warn("Message rejected by BeforeMessage hook",
					"clientID", c.ID,
					"error", err,
				)
				continue
			}
			if modifiedMsg != nil {
				msg = modifiedMsg
			}
		}

		// Call client-specific handler
		c.callbackMu.RLock()
		messageHandler := c.onMessage
		c.callbackMu.RUnlock()

		if messageHandler != nil {
			messageHandler(c, msg)
		}

		// Call hub-level handler with latency recording
		var handlerErr error
		if c.hub.onMessage != nil {
			start := time.Now()
			handlerErr = c.hub.onMessage(c, msg)
			c.hub.metrics.RecordLatency(time.Since(start))
		}

		// Call AfterMessage hook
		if c.hub.hooks.AfterMessage != nil {
			c.hub.hooks.AfterMessage(c, msg, handlerErr)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(c.config.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case item, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(item.msgType, item.data); err != nil {
				c.hub.metrics.IncrementErrors("write_error")
				return
			}

			// Send queued messages as separate frames
			n := len(c.send)
			for range n {
				queued := <-c.send
				if err := c.conn.WriteMessage(queued.msgType, queued.data); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.hub.metrics.IncrementErrors("ping_error")
				return
			}
		}
	}
}
