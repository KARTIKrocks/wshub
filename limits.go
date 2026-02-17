package websocket

// Limits defines various limits for WebSocket operations.
type Limits struct {
	// MaxConnections is the maximum number of concurrent connections.
	// 0 means unlimited.
	MaxConnections int

	// MaxConnectionsPerUser is the maximum number of connections per user ID.
	// 0 means unlimited.
	MaxConnectionsPerUser int

	// MaxRoomsPerClient is the maximum number of rooms a client can join.
	// 0 means unlimited.
	MaxRoomsPerClient int

	// MaxClientsPerRoom is the maximum number of clients in a room.
	// 0 means unlimited.
	MaxClientsPerRoom int

	// MaxMessageRate is the maximum messages per second per client.
	// 0 means unlimited.
	MaxMessageRate int
}

// DefaultLimits returns default limits (all unlimited).
func DefaultLimits() Limits {
	return Limits{
		MaxConnections:        0,
		MaxConnectionsPerUser: 0,
		MaxRoomsPerClient:     0,
		MaxClientsPerRoom:     0,
		MaxMessageRate:        0,
	}
}

// WithMaxConnections sets the maximum connections limit.
func (l Limits) WithMaxConnections(max int) Limits {
	l.MaxConnections = max
	return l
}

// WithMaxConnectionsPerUser sets the maximum connections per user limit.
func (l Limits) WithMaxConnectionsPerUser(max int) Limits {
	l.MaxConnectionsPerUser = max
	return l
}

// WithMaxRoomsPerClient sets the maximum rooms per client limit.
func (l Limits) WithMaxRoomsPerClient(max int) Limits {
	l.MaxRoomsPerClient = max
	return l
}

// WithMaxClientsPerRoom sets the maximum clients per room limit.
func (l Limits) WithMaxClientsPerRoom(max int) Limits {
	l.MaxClientsPerRoom = max
	return l
}

// WithMaxMessageRate sets the maximum message rate limit.
func (l Limits) WithMaxMessageRate(rate int) Limits {
	l.MaxMessageRate = rate
	return l
}
