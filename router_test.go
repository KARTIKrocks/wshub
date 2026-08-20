package wshub

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
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
	t.Parallel()
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
	t.Parallel()
	router := NewRouter(jsonExtractor)

	msg := &Message{Data: []byte(`{"type":"unknown"}`)}
	err := router.Handle(nil, msg)
	if !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("err = %v, want ErrInvalidMessage", err)
	}
}

func TestRouterCustomNotFound(t *testing.T) {
	t.Parallel()
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
	var invocations atomic.Int64
	var errCount atomic.Int64

	router := NewRouter(jsonExtractor).
		On("ping", func(c *Client, m *Message) error {
			invocations.Add(1)
			return nil
		})

	msg := &Message{Data: []byte(`{"type":"ping"}`)}

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if err := router.Handle(nil, msg); err != nil {
				errCount.Add(1)
			}
		})
	}
	wg.Wait()

	if invocations.Load() != 100 {
		t.Errorf("handler invocations = %d, want 100", invocations.Load())
	}
	if errCount.Load() != 0 {
		t.Errorf("error count = %d, want 0", errCount.Load())
	}
}
