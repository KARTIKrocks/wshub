package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents a WebSocket client connection.
type Client struct {
	// ID is the unique identifier for this client.
	ID string

	// UserID is an optional user identifier (set after authentication).
	UserID string

	// Metadata stores custom data associated with the client.
	Metadata map[string]any

	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	config Config

	mu       sync.RWMutex
	rooms    map[string]bool
	closed   bool
	closedAt time.Time

	// Callbacks protected by callbackMu
	callbackMu sync.RWMutex
	onMessage  func(*Client, *Message)
	onClose    func(*Client)
	onError    func(*Client, error)
}

// newClient creates a new client from a WebSocket connection.
func newClient(hub *Hub, conn *websocket.Conn, config Config) *Client {
	return &Client{
		ID:       uuid.New().String(),
		Metadata: make(map[string]any),
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, config.SendChannelSize),
		config:   config,
		rooms:    make(map[string]bool),
	}
}

// SetUserID sets the user ID for authenticated clients.
func (c *Client) SetUserID(userID string) {
	c.mu.Lock()
	oldUserID := c.UserID
	c.UserID = userID
	c.mu.Unlock()

	// Update the hub's user index
	c.hub.UpdateClientUserID(c, oldUserID, userID)
}

// GetUserID returns the user ID.
func (c *Client) GetUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.UserID
}

// SetMetadata sets a metadata value.
func (c *Client) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Metadata[key] = value
}

// GetMetadata returns a metadata value.
func (c *Client) GetMetadata(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.Metadata[key]
	return v, ok
}

// DeleteMetadata removes a metadata value.
func (c *Client) DeleteMetadata(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Metadata, key)
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
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Send(data)
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

	select {
	case c.send <- data:
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

	select {
	case c.send <- data:
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
	return c.rooms[room]
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
	c.rooms[room] = true
}

// leaveRoom marks the client as left from a room.
func (c *Client) leaveRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, room)
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
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

			// Call hub-level handler
			var handlerErr error
			if c.hub.onMessage != nil {
				handlerErr = c.hub.onMessage(c, msg)
			}

			// Call AfterMessage hook
			if c.hub.hooks.AfterMessage != nil {
				c.hub.hooks.AfterMessage(c, msg, handlerErr)
			}
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
			/*
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.hub.metrics.IncrementErrors("write_error")
				return
			}

			// Send queued messages as separate frames
			n := len(c.send)
			for range n {
				if err := c.conn.WriteMessage(websocket.TextMessage, <-c.send); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return
				}
			}

			 */
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.hub.metrics.IncrementErrors("write_error")
				return
			}
			w.Write(message)

			// Batch queued messages
			n := len(c.send)
			for range n {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				c.hub.metrics.IncrementErrors("write_close_error")
				return
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
