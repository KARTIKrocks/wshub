package wshub

import (
	"encoding/json"
	"time"
)

// Message represents a WebSocket message.
type Message struct {
	// Type is the message type (text, binary, etc.).
	Type MessageType

	// Data is the raw message data.
	Data []byte

	// ClientID is the ID of the client that sent the message.
	ClientID string

	// Time is when the message was received.
	Time time.Time
}

// Text returns the message data as a string.
func (m *Message) Text() string {
	return string(m.Data)
}

// JSON unmarshals the message data into the provided value.
func (m *Message) JSON(v any) error {
	return json.Unmarshal(m.Data, v)
}

// NewMessage creates a new text message.
func NewMessage(data []byte) *Message {
	return &Message{
		Type: TextMessage,
		Data: data,
		Time: time.Now(),
	}
}

// NewTextMessage creates a new text message from a string.
func NewTextMessage(text string) *Message {
	return &Message{
		Type: TextMessage,
		Data: []byte(text),
		Time: time.Now(),
	}
}

// NewBinaryMessage creates a new binary message.
func NewBinaryMessage(data []byte) *Message {
	return &Message{
		Type: BinaryMessage,
		Data: data,
		Time: time.Now(),
	}
}

// NewJSONMessage creates a new JSON message.
func NewJSONMessage(v any) (*Message, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type: TextMessage,
		Data: data,
		Time: time.Now(),
	}, nil
}
