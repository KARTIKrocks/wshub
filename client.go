package wshub

import (
	"context"
	"net/http"
	"sync"
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

	// sendMu serializes DropOldest evict+enqueue so the two channel
	// operations are atomic with respect to other writers.
	sendMu sync.Mutex

	// Token-bucket rate limiting — protected by rateMu
	rateMu     sync.Mutex
	tokens     float64
	lastRefill time.Time

	// Close code/reason set by CloseWithCode for writePump to send.
	closeCode   int
	closeReason string

	// Close-once guard
	closeOnce sync.Once

	// done is closed to signal writePump to exit when the client is
	// unregistered (remote/abnormal close). CloseWithCode uses
	// close(c.send) instead, so writePump can send the close frame.
	done     chan struct{}
	doneOnce sync.Once

	// Callbacks protected by callbackMu
	callbackMu sync.RWMutex
	onMessage  func(*Client, *Message)
	onClose    func(*Client)
	onError    func(*Client, error)
}

// newClient creates a new client from a WebSocket connection.
func newClient(hub *Hub, conn *websocket.Conn, config Config, r *http.Request) *Client {
	maxRate := float64(hub.limits.MaxMessageRate)
	if maxRate <= 0 {
		maxRate = 1 // unused when rate limiting is disabled
	}
	now := time.Now()
	c := &Client{
		ID:          uuid.New().String(),
		hub:         hub,
		conn:        conn,
		send:        make(chan sendItem, config.SendChannelSize),
		done:        make(chan struct{}),
		config:      config,
		rooms:       make(map[string]struct{}),
		connectedAt: now,
		request:     r,
		tokens:      maxRate, // start with a full bucket
		lastRefill:  now,
	}
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
	return c.hub.setClientUserID(c, userID)
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
	if c.metadata == nil {
		c.metadata = make(map[string]any)
	}
	c.metadata[key] = value
}

// GetMetadata returns a metadata value.
func (c *Client) GetMetadata(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.metadata == nil {
		return nil, false
	}
	v, ok := c.metadata[key]
	return v, ok
}

// DeleteMetadata removes a metadata value.
func (c *Client) DeleteMetadata(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.metadata, key) // no-op on nil map
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

// SendRawJSON sends pre-encoded JSON data as a text message to the client.
// Use this instead of SendJSON when the JSON is already marshaled to avoid
// redundant serialization.
func (c *Client) SendRawJSON(data []byte) error {
	return c.Send(data)
}

// SendBinary sends a binary message to the client.
func (c *Client) SendBinary(data []byte) error {
	return c.SendMessage(BinaryMessage, data)
}

// SendMessage sends a message with the specified type. The behavior when
// the send buffer is full depends on the hub's DropPolicy.
func (c *Client) SendMessage(msgType MessageType, data []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	item := sendItem{msgType: int(msgType), data: data}
	if ok := c.hub.trySendErr(c, item); ok {
		return nil
	}
	return ErrSendBufferFull
}

// SendWithContext sends a text message with context support.
// It blocks until the message is enqueued or the context is cancelled.
func (c *Client) SendWithContext(ctx context.Context, data []byte) error {
	return c.SendMessageWithContext(ctx, TextMessage, data)
}

// SendMessageWithContext sends a message with the specified type and context support.
// It blocks until the message is enqueued or the context is cancelled.
//
// Unlike SendMessage, this method does not apply the hub's DropPolicy.
// When the send buffer is full it waits for space rather than dropping
// messages, giving callers explicit control over the timeout via ctx.
func (c *Client) SendMessageWithContext(ctx context.Context, msgType MessageType, data []byte) (err error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	// Recover from sending on a closed channel if the client disconnects
	// concurrently (CloseWithCode closes c.send). Re-panic on anything else
	// so that real bugs are not silently swallowed.
	defer func() {
		if r := recover(); r != nil {
			if isChanSendPanic(r) {
				err = ErrConnectionClosed
			} else {
				panic(r)
			}
		}
	}()

	item := sendItem{msgType: int(msgType), data: data}
	select {
	case c.send <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.CloseWithCode(websocket.CloseNormalClosure, "")
}

// CloseWithCode closes the client connection with a specific close code and reason.
// It uses mu+closed rather than sync.Once because it needs to atomically
// set closeCode/closeReason and the closed flag before closing the send channel.
func (c *Client) CloseWithCode(code int, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.closedAt = time.Now()
	c.closeCode = code
	c.closeReason = reason
	c.mu.Unlock()

	// Closing the send channel signals writePump to send the close frame
	// and exit. writePump and readPump both call closeConn() in their
	// defers, so the underlying connection is always cleaned up.
	// We must NOT call closeConn() here — doing so would race with
	// writePump trying to send the close frame.
	close(c.send)
	return nil
}

// closeDone signals writePump to exit by closing the done channel.
// Used by handleUnregister (remote/abnormal close) where we don't need
// to send a close frame. Safe for concurrent calls via sync.Once.
func (c *Client) closeDone() {
	c.doneOnce.Do(func() {
		close(c.done)
	})
}

// closeConn closes the underlying WebSocket connection exactly once.
func (c *Client) closeConn() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
	})
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

// leaveRoom marks the client as left from a room.
func (c *Client) leaveRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, room)
}

// checkRateLimit checks if the client has exceeded the message rate limit
// using a token-bucket algorithm. Each message costs one token; tokens refill
// at MaxMessageRate per second up to a burst of MaxMessageRate.
// Returns true if the message should be allowed.
func (c *Client) checkRateLimit() bool {
	maxRate := c.hub.limits.MaxMessageRate
	if maxRate <= 0 {
		return true
	}

	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	now := time.Now()
	elapsed := now.Sub(c.lastRefill).Seconds()
	c.lastRefill = now

	rate := float64(maxRate)
	c.tokens += elapsed * rate
	if c.tokens > rate {
		c.tokens = rate // cap at burst size
	}

	if c.tokens < 1 {
		return false
	}
	c.tokens--
	return true
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump(ctx context.Context) {
	done := make(chan struct{})
	defer func() {
		close(done)
		// Use select to avoid blocking forever if Run() has already exited.
		select {
		case c.hub.unregister <- c:
		case <-c.hub.ctx.Done():
		}
		c.closeConn()
	}()

	// Use a goroutine watching ctx.Done() to close the connection and unblock ReadMessage.
	go func() {
		select {
		case <-ctx.Done():
			c.closeConn()
		case <-done:
		}
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait)); err != nil {
		c.hub.metrics.IncrementErrors("read_deadline_error")
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	})

	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			c.handleReadError(err)
			return
		}

		// Check rate limit before counting the message in metrics,
		// so dropped messages don't inflate counters.
		if !c.checkRateLimit() {
			c.hub.logger.Warn("Rate limit exceeded, dropping message",
				"clientID", c.ID,
			)
			c.hub.metrics.IncrementErrors("rate_limit_exceeded")
			continue
		}

		c.processMessage(messageType, data)
	}
}

// handleReadError reports unexpected close errors to callbacks, hooks, and metrics.
func (c *Client) handleReadError(err error) {
	if !websocket.IsUnexpectedCloseError(err,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure,
		websocket.CloseNormalClosure) {
		return
	}

	c.callbackMu.RLock()
	errorHandler := c.onError
	c.callbackMu.RUnlock()

	if errorHandler != nil {
		errorHandler(c, err)
	}

	if c.hub.hooks.OnError != nil {
		c.hub.hooks.OnError(c, err)
	}

	c.hub.metrics.IncrementErrors("read_error")
}

// processMessage runs hooks, client/hub handlers, and metrics for an accepted message.
func (c *Client) processMessage(messageType int, data []byte) {
	now := time.Now()
	c.hub.metrics.IncrementMessagesReceived()
	c.hub.metrics.RecordMessageSize(len(data))

	msg := &Message{
		Type:     MessageType(messageType),
		Data:     data,
		ClientID: c.ID,
		Time:     now,
	}

	// Call BeforeMessage hook
	if c.hub.hooks.BeforeMessage != nil {
		modifiedMsg, err := c.hub.hooks.BeforeMessage(c, msg)
		if err != nil {
			c.hub.logger.Warn("Message rejected by BeforeMessage hook",
				"clientID", c.ID,
				"error", err,
			)
			return
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

	// Call hub-level handler with latency recording.
	// Latency is skipped when WithoutHandlerLatency is set to avoid
	// double-counting with MetricsMiddleware.
	var handlerErr error
	if c.hub.onMessage != nil {
		start := time.Now()
		handlerErr = c.hub.onMessage(c, msg)
		if !c.hub.skipHandlerLatency {
			c.hub.metrics.RecordLatency(time.Since(start))
		}
	}

	// Call AfterMessage hook
	if c.hub.hooks.AfterMessage != nil {
		c.hub.hooks.AfterMessage(c, msg, handlerErr)
	}
}

// writeCloseFrame sends a WebSocket close frame with the client's close code
// and reason. Called by writePump when the send channel is closed.
func (c *Client) writeCloseFrame() {
	c.mu.RLock()
	code, reason := c.closeCode, c.closeReason
	c.mu.RUnlock()

	var msg []byte
	if code != 0 {
		msg = websocket.FormatCloseMessage(code, reason)
	}
	_ = c.conn.WriteMessage(websocket.CloseMessage, msg)
}

// drainQueued writes any messages buffered in the send channel.
// Returns false if the send channel was closed or a write error
// occurred, signalling writePump to exit.
func (c *Client) drainQueued() bool {
	n := len(c.send)
	if n == 0 {
		return true
	}
	// Extend the write deadline to cover the queued batch.
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
		c.hub.metrics.IncrementErrors("write_deadline_error")
		return false
	}
	for range n {
		select {
		case queued, ok := <-c.send:
			if !ok {
				return false
			}
			if err := c.conn.WriteMessage(queued.msgType, queued.data); err != nil {
				c.hub.metrics.IncrementErrors("write_error")
				return false
			}
			c.hub.metrics.IncrementMessagesSent(1)
		default:
			return true
		}
	}
	return true
}

// drainQueuedCoalesced writes the first item and any queued items,
// coalescing consecutive text messages into a single WebSocket frame
// separated by newline bytes. Non-text messages are written individually.
// Returns false if the send channel was closed or a write error occurred.
func (c *Client) drainQueuedCoalesced(first sendItem) bool {
	n := len(c.send)

	// Fast path: nothing queued or first message is non-text.
	if n == 0 || first.msgType != websocket.TextMessage {
		if err := c.conn.WriteMessage(first.msgType, first.data); err != nil {
			c.hub.metrics.IncrementErrors("write_error")
			return false
		}
		c.hub.metrics.IncrementMessagesSent(1)
		if n == 0 {
			return true
		}
		return c.drainQueued()
	}

	return c.writeCoalescedBatch(first, n)
}

// writeCoalescedBatch opens a single NextWriter and coalesces the first text
// message with up to n queued text messages into one frame. A non-text item
// in the queue flushes the current frame and is written individually.
func (c *Client) writeCoalescedBatch(first sendItem, n int) bool {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
		c.hub.metrics.IncrementErrors("write_deadline_error")
		return false
	}

	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		c.hub.metrics.IncrementErrors("write_error")
		return false
	}

	if _, err := w.Write(first.data); err != nil {
		c.hub.metrics.IncrementErrors("write_error")
		return false
	}
	sentCount := 1

	for range n {
		select {
		case queued, ok := <-c.send:
			if !ok {
				_ = w.Close()
				return false
			}
			if queued.msgType == websocket.TextMessage {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return false
				}
				if _, err := w.Write(queued.data); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return false
				}
				sentCount++
			} else {
				// Type changed: flush coalesced text, write non-text individually.
				if err := w.Close(); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return false
				}
				c.hub.metrics.IncrementMessagesSent(sentCount)
				if err := c.conn.WriteMessage(queued.msgType, queued.data); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return false
				}
				c.hub.metrics.IncrementMessagesSent(1)
				return true
			}
		default:
			if err := w.Close(); err != nil {
				c.hub.metrics.IncrementErrors("write_error")
				return false
			}
			c.hub.metrics.IncrementMessagesSent(sentCount)
			return true
		}
	}

	if err := w.Close(); err != nil {
		c.hub.metrics.IncrementErrors("write_error")
		return false
	}
	c.hub.metrics.IncrementMessagesSent(sentCount)
	return true
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(c.config.PingPeriod)
	defer func() {
		ticker.Stop()
		c.closeConn()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case <-c.done:
			// Client was unregistered (remote/abnormal close). Exit
			// without sending a close frame — the connection is gone.
			return

		case item, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
				c.hub.metrics.IncrementErrors("write_deadline_error")
				return
			}
			if !ok {
				c.writeCloseFrame()
				return
			}
			if c.config.CoalesceWrites {
				if !c.drainQueuedCoalesced(item) {
					return
				}
			} else {
				if err := c.conn.WriteMessage(item.msgType, item.data); err != nil {
					c.hub.metrics.IncrementErrors("write_error")
					return
				}
				c.hub.metrics.IncrementMessagesSent(1)
				if !c.drainQueued() {
					return
				}
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
				c.hub.metrics.IncrementErrors("write_deadline_error")
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.hub.metrics.IncrementErrors("ping_error")
				return
			}
		}
	}
}
