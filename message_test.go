package wshub

import (
	"testing"
)

func TestMessage_Text(t *testing.T) {
	msg := &Message{Data: []byte("hello")}
	if got := msg.Text(); got != "hello" {
		t.Errorf("Text() = %q, want %q", got, "hello")
	}
}

func TestMessage_Text_Empty(t *testing.T) {
	msg := &Message{Data: []byte{}}
	if got := msg.Text(); got != "" {
		t.Errorf("Text() = %q, want empty string", got)
	}
}

func TestMessage_JSON(t *testing.T) {
	msg := &Message{Data: []byte(`{"name":"test","value":42}`)}
	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := msg.JSON(&result); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if result.Name != "test" || result.Value != 42 {
		t.Errorf("JSON() got %+v, want {Name:test Value:42}", result)
	}
}

func TestMessage_JSON_Invalid(t *testing.T) {
	msg := &Message{Data: []byte("not json")}
	var result map[string]any
	if err := msg.JSON(&result); err == nil {
		t.Error("JSON() expected error for invalid JSON")
	}
}

func TestNewMessage(t *testing.T) {
	data := []byte("hello")
	msg := NewMessage(data)
	if msg.Type != TextMessage {
		t.Errorf("Type = %v, want TextMessage", msg.Type)
	}
	if string(msg.Data) != "hello" {
		t.Errorf("Data = %q, want %q", msg.Data, "hello")
	}
	if msg.Time.IsZero() {
		t.Error("Time should not be zero")
	}
}

func TestNewTextMessage(t *testing.T) {
	msg := NewTextMessage("hello")
	if msg.Type != TextMessage {
		t.Errorf("Type = %v, want TextMessage", msg.Type)
	}
	if string(msg.Data) != "hello" {
		t.Errorf("Data = %q, want %q", msg.Data, "hello")
	}
	if msg.Time.IsZero() {
		t.Error("Time should not be zero")
	}
}

func TestNewBinaryMessage(t *testing.T) {
	data := []byte{0x00, 0xFF, 0x42}
	msg := NewBinaryMessage(data)
	if msg.Type != BinaryMessage {
		t.Errorf("Type = %v, want BinaryMessage", msg.Type)
	}
	if len(msg.Data) != 3 || msg.Data[2] != 0x42 {
		t.Errorf("Data = %v, want %v", msg.Data, data)
	}
	if msg.Time.IsZero() {
		t.Error("Time should not be zero")
	}
}

func TestNewJSONMessage(t *testing.T) {
	input := map[string]string{"key": "value"}
	msg, err := NewJSONMessage(input)
	if err != nil {
		t.Fatalf("NewJSONMessage() error = %v", err)
	}
	if msg.Type != TextMessage {
		t.Errorf("Type = %v, want TextMessage", msg.Type)
	}
	if msg.Time.IsZero() {
		t.Error("Time should not be zero")
	}

	var result map[string]string
	if err := msg.JSON(&result); err != nil {
		t.Fatalf("JSON() roundtrip error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("roundtrip got %v, want map[key:value]", result)
	}
}

func TestNewJSONMessage_Error(t *testing.T) {
	// Channels cannot be marshaled to JSON
	_, err := NewJSONMessage(make(chan int))
	if err == nil {
		t.Error("NewJSONMessage() expected error for unmarshallable type")
	}
}

func TestNewRawJSONMessage(t *testing.T) {
	data := []byte(`{"key":"value"}`)
	msg := NewRawJSONMessage(data)
	if msg.Type != TextMessage {
		t.Errorf("Type = %v, want TextMessage", msg.Type)
	}
	if string(msg.Data) != `{"key":"value"}` {
		t.Errorf("Data = %q, want %q", msg.Data, `{"key":"value"}`)
	}
	if msg.Time.IsZero() {
		t.Error("Time should not be zero")
	}
}
