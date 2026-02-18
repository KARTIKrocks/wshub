package wshub

import "net/http"

// Hooks defines lifecycle callbacks for WebSocket operations.
type Hooks struct {
	// BeforeConnect is called before upgrading the connection.
	// Return an error to reject the connection.
	BeforeConnect func(*http.Request) error

	// AfterConnect is called after a client successfully connects.
	AfterConnect func(*Client)

	// BeforeDisconnect is called before a client disconnects.
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
