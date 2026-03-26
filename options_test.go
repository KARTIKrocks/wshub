package wshub

import (
	"runtime"
	"testing"
	"time"
)

func TestWithConfig(t *testing.T) {
	cfg := Config{
		WriteWait:  5 * time.Second,
		PongWait:   30 * time.Second,
		PingPeriod: 25 * time.Second,
	}
	hub := NewHub(WithConfig(cfg))
	if hub.config.WriteWait != 5*time.Second {
		t.Errorf("WriteWait = %v, want 5s", hub.config.WriteWait)
	}
	if hub.config.PongWait != 30*time.Second {
		t.Errorf("PongWait = %v, want 30s", hub.config.PongWait)
	}
}

func TestWithHooks(t *testing.T) {
	called := false
	hooks := Hooks{
		AfterConnect: func(c *Client) {
			called = true
		},
	}
	hub := NewHub(WithHooks(hooks))
	if hub.hooks.AfterConnect == nil {
		t.Error("AfterConnect hook should be set")
	}
	hub.hooks.AfterConnect(nil)
	if !called {
		t.Error("AfterConnect hook was not called")
	}
}

func TestWithParallelBroadcast(t *testing.T) {
	hub := NewHub(WithParallelBroadcast(100))
	if !hub.useParallel {
		t.Error("useParallel should be true")
	}
	if hub.parallelBatchSize != 100 {
		t.Errorf("parallelBatchSize = %d, want 100", hub.parallelBatchSize)
	}
}

func TestWithParallelBroadcast_ZeroBatchSize(t *testing.T) {
	hub := NewHub(WithParallelBroadcast(0))
	if !hub.useParallel {
		t.Error("useParallel should be true")
	}
	// batchSize <= 0 should not override the default
	if hub.parallelBatchSize == 0 {
		t.Error("parallelBatchSize should keep default when 0 is passed")
	}
}

func TestWithParallelBroadcastWorkers(t *testing.T) {
	hub := NewHub(WithParallelBroadcast(100), WithParallelBroadcastWorkers(8))
	if hub.poolSize != 8 {
		t.Errorf("poolSize = %d, want 8", hub.poolSize)
	}
}

func TestWithParallelBroadcastWorkers_Zero(t *testing.T) {
	hub := NewHub(WithParallelBroadcastWorkers(0))
	if hub.poolSize != runtime.NumCPU() {
		t.Errorf("poolSize = %d, want default %d", hub.poolSize, runtime.NumCPU())
	}
}

func TestWithMessageHandler(t *testing.T) {
	called := false
	handler := func(c *Client, m *Message) error {
		called = true
		return nil
	}
	hub := NewHub(WithMessageHandler(handler))
	if hub.onMessage == nil {
		t.Error("onMessage should be set")
	}
	_ = hub.onMessage(nil, nil)
	if !called {
		t.Error("message handler was not called")
	}
}
