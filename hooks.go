package wshub

import "net/http"

// Hooks defines lifecycle callbacks for WebSocket operations.
type Hooks struct {
	// BeforeConnect is called before upgrading the connection.
	// Return an error to reject the connection.
	BeforeConnect func(*http.Request) error

	// AfterConnect is called after a client successfully connects.
	// It runs in its own goroutine, so the client may receive messages
	// before this callback returns.
	AfterConnect func(*Client)

	// BeforeDisconnect is called before a client disconnects.
	// The hook runs in a goroutine with a timeout (default 5s, configurable
	// via WithHookTimeout). If the hook does not complete within the timeout
	// the hub proceeds with the disconnect; the goroutine is NOT cancelled
	// and will continue running until it returns. Keep this hook fast to
	// avoid leaking goroutines.
	BeforeDisconnect func(*Client)

	// AfterDisconnect is called after a client disconnects.
	AfterDisconnect func(*Client)

	// BeforeMessage is called before processing a message.
	// Can modify the message or return an error to reject it.
	BeforeMessage func(*Client, *Message) (*Message, error)

	// AfterMessage is called after processing a message.
	AfterMessage func(*Client, *Message, error)

	// OnError is called when an error occurs.
	OnError func(*Client, error)

	// OnSendDropped is called when a message is dropped because the client's
	// send buffer is full. The application can use this to decide whether to
	// disconnect the slow client, log the event, or queue the data externally.
	// The hook is called synchronously in the sender's goroutine — keep it
	// fast to avoid blocking broadcasts.
	OnSendDropped func(client *Client, data []byte)

	// BeforeRoomJoin is called before a client joins a room.
	// Return an error to prevent joining.
	BeforeRoomJoin func(*Client, string) error

	// AfterRoomJoin is called after a client joins a room.
	AfterRoomJoin func(*Client, string)

	// BeforeRoomLeave is called before a client leaves a room.
	BeforeRoomLeave func(*Client, string)

	// AfterRoomLeave is called after a client leaves a room.
	AfterRoomLeave func(*Client, string)
}
