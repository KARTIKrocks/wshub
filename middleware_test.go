package wshub

import (
	"errors"
	"testing"
)

func TestMiddlewareChainExecutionOrder(t *testing.T) {
	var order []int

	handler := func(c *Client, m *Message) error {
		order = append(order, 0)
		return nil
	}

	mw1 := func(next HandlerFunc) HandlerFunc {
		return func(c *Client, m *Message) error {
			order = append(order, 1)
			return next(c, m)
		}
	}

	mw2 := func(next HandlerFunc) HandlerFunc {
		return func(c *Client, m *Message) error {
			order = append(order, 2)
			return next(c, m)
		}
	}

	chain := NewMiddlewareChain(handler).Use(mw1).Use(mw2)
	err := chain.Execute(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// mw1 wraps mw2 wraps handler: execution order should be 1, 2, 0
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 0 {
		t.Errorf("execution order = %v, want [1 2 0]", order)
	}
}

func TestMiddlewareChainBuild(t *testing.T) {
	callCount := 0

	handler := func(c *Client, m *Message) error {
		callCount++
		return nil
	}

	chain := NewMiddlewareChain(handler).Build()

	if chain.built == nil {
		t.Fatal("Build should set the built field")
	}

	chain.Execute(nil, nil)
	chain.Execute(nil, nil)

	if callCount != 2 {
		t.Errorf("handler called %d times, want 2", callCount)
	}
}

func TestMiddlewareChainBuildInvalidation(t *testing.T) {
	handler := func(c *Client, m *Message) error { return nil }

	chain := NewMiddlewareChain(handler).Build()
	if chain.built == nil {
		t.Fatal("Build should cache")
	}

	// Adding middleware should invalidate cache
	chain.Use(func(next HandlerFunc) HandlerFunc { return next })
	if chain.built != nil {
		t.Error("Use should invalidate built cache")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	logger := &NoOpLogger{}

	handler := func(c *Client, m *Message) error {
		panic("test panic")
	}

	// Create a minimal client for the test
	hub := NewHub()
	c := &Client{
		ID:       "test",
		hub:      hub,
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	chain := NewMiddlewareChain(handler).
		Use(RecoveryMiddleware(logger)).
		Build()

	err := chain.Execute(c, &Message{})
	if err == nil {
		t.Fatal("expected error from recovered panic")
	}
	if !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("err = %v, want ErrInvalidMessage", err)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger := &NoOpLogger{}
	called := false

	handler := func(c *Client, m *Message) error {
		called = true
		return nil
	}

	// Create a minimal client for the test
	hub := NewHub()
	c := &Client{
		ID:       "test",
		hub:      hub,
		metadata: make(map[string]any),
		rooms:    make(map[string]struct{}),
	}

	chain := NewMiddlewareChain(handler).
		Use(LoggingMiddleware(logger)).
		Build()

	chain.Execute(c, &Message{Data: []byte("hello")})

	if !called {
		t.Error("handler was not called through logging middleware")
	}
}

func TestMetricsMiddleware(t *testing.T) {
	metrics := NewDebugMetrics()

	handler := func(c *Client, m *Message) error {
		return errors.New("test error")
	}

	chain := NewMiddlewareChain(handler).
		Use(MetricsMiddleware(metrics)).
		Build()

	chain.Execute(nil, &Message{Data: []byte("hello")})

	s := metrics.Stats()
	// MetricsMiddleware no longer counts messages (readPump does that);
	// it records latency and handler errors.
	if s.AvgLatency <= 0 {
		t.Error("AvgLatency should be > 0 after executing a handler")
	}
	if s.Errors["message_handling"] != 1 {
		t.Errorf("message_handling errors = %d, want 1", s.Errors["message_handling"])
	}
}

func TestMiddlewareChainNoMiddleware(t *testing.T) {
	called := false
	handler := func(c *Client, m *Message) error {
		called = true
		return nil
	}

	chain := NewMiddlewareChain(handler)
	chain.Execute(nil, nil)

	if !called {
		t.Error("handler should be called even with no middleware")
	}
}
