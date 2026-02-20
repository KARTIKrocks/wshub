package wshub

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func jsonExtractor(msg *Message) string {
	var env struct {
		Type string `json:"type"`
	}
	json.Unmarshal(msg.Data, &env)
	return env.Type
}

func TestRouterDispatch(t *testing.T) {
	chatCalled := false
	joinCalled := false

	router := NewRouter(jsonExtractor).
		On("chat", func(c *Client, m *Message) error {
			chatCalled = true
			return nil
		}).
		On("join", func(c *Client, m *Message) error {
			joinCalled = true
			return nil
		})

	chatMsg := &Message{Data: []byte(`{"type":"chat","message":"hi"}`)}
	err := router.Handle(nil, chatMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !chatCalled {
		t.Error("chat handler was not called")
	}

	joinMsg := &Message{Data: []byte(`{"type":"join"}`)}
	err = router.Handle(nil, joinMsg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !joinCalled {
		t.Error("join handler was not called")
	}
}

func TestRouterNotFound(t *testing.T) {
	router := NewRouter(jsonExtractor)

	msg := &Message{Data: []byte(`{"type":"unknown"}`)}
	err := router.Handle(nil, msg)
	if !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("err = %v, want ErrInvalidMessage", err)
	}
}

func TestRouterCustomNotFound(t *testing.T) {
	notFoundCalled := false

	router := NewRouter(jsonExtractor).
		OnNotFound(func(c *Client, m *Message) error {
			notFoundCalled = true
			return nil
		})

	msg := &Message{Data: []byte(`{"type":"unknown"}`)}
	err := router.Handle(nil, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !notFoundCalled {
		t.Error("not-found handler was not called")
	}
}

func TestRouterConcurrentAccess(t *testing.T) {
	router := NewRouter(jsonExtractor).
		On("ping", func(c *Client, m *Message) error {
			return nil
		})

	msg := &Message{Data: []byte(`{"type":"ping"}`)}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			router.Handle(nil, msg)
		}()
	}
	wg.Wait()
}
