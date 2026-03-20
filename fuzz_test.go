package wshub

import (
	"encoding/json"
	"testing"
)

func FuzzMessageJSON(f *testing.F) {
	// Seed corpus
	f.Add([]byte(`{"type":"chat","text":"hello"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"nested":{"a":1}}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(`"just a string"`))
	f.Add([]byte(`12345`))
	f.Add([]byte{})
	f.Add([]byte(`{invalid json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := &Message{Data: data, Type: TextMessage}

		// Text() should never panic
		_ = msg.Text()

		// JSON() should not panic regardless of input
		var result any
		_ = msg.JSON(&result)
	})
}

func FuzzNewJSONMessage(f *testing.F) {
	f.Add(`{"key":"value"}`)
	f.Add(`hello world`)
	f.Add(``)
	f.Add(`{"emoji":"😀","nested":{"arr":[1,2,3]}}`)

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic, may return error
		msg, err := NewJSONMessage(input)
		if err != nil {
			return
		}

		// If it succeeded, Data should be valid JSON
		if !json.Valid(msg.Data) {
			t.Errorf("NewJSONMessage produced invalid JSON: %q", msg.Data)
		}
	})
}

func FuzzRouterDispatch(f *testing.F) {
	f.Add([]byte(`{"event":"chat","data":"hello"}`))
	f.Add([]byte(`{"event":"","data":""}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte{})
	f.Add([]byte(`{"event":123}`))

	router := NewRouter(func(m *Message) string {
		var envelope struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(m.Data, &envelope); err != nil {
			return ""
		}
		return envelope.Event
	})

	router.On("chat", func(c *Client, m *Message) error {
		return nil
	})
	router.OnNotFound(func(c *Client, m *Message) error {
		return nil
	})

	hub := NewHub()
	client := &Client{
		ID:       "fuzz-client",
		hub:      hub,
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := &Message{Data: data, Type: TextMessage, ClientID: "fuzz-client"}
		// Should never panic
		_ = router.Handle(client, msg)
	})
}

func FuzzMiddlewareChain(f *testing.F) {
	f.Add([]byte("normal message"))
	f.Add([]byte{})
	f.Add(make([]byte, 65536))

	hub := NewHub()
	chain := NewMiddlewareChain(func(c *Client, m *Message) error {
		return nil
	}).
		Use(RecoveryMiddleware(&NoOpLogger{})).
		Build()

	client := &Client{
		ID:       "fuzz-mw",
		hub:      hub,
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := &Message{Data: data, Type: TextMessage, ClientID: "fuzz-mw"}
		// Should never panic due to recovery middleware
		_ = chain.Execute(client, msg)
	})
}
