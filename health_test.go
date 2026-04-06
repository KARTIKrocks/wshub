package wshub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestAlive_BeforeRun(t *testing.T) {
	hub := NewHub()
	if hub.Alive() {
		t.Fatal("Alive() should be false before Run()")
	}
	if hub.Ready() {
		t.Fatal("Ready() should be false before Run()")
	}
}

func TestAlive_AfterRun(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	time.Sleep(50 * time.Millisecond)

	if !hub.Alive() {
		t.Fatal("Alive() should be true after Run()")
	}
	if !hub.Ready() {
		t.Fatal("Ready() should be true after Run()")
	}
}

func TestAlive_AfterShutdown(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	hub.Shutdown(ctx)

	if hub.Alive() {
		t.Fatal("Alive() should be false after Shutdown()")
	}
	if hub.Ready() {
		t.Fatal("Ready() should be false after Shutdown()")
	}
}

func TestReady_WhileDraining(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	time.Sleep(50 * time.Millisecond)

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer drainCancel()
	hub.Drain(drainCtx)

	if !hub.Alive() {
		t.Fatal("Alive() should be true while draining (Run loop still active)")
	}
	if hub.Ready() {
		t.Fatal("Ready() should be false while draining")
	}
}

func TestUptime_BeforeRun(t *testing.T) {
	hub := NewHub()
	if got := hub.Uptime(); got != 0 {
		t.Fatalf("Uptime() = %v before Run(), want 0", got)
	}
}

func TestUptime_AfterRun(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	time.Sleep(100 * time.Millisecond)

	if got := hub.Uptime(); got < 50*time.Millisecond {
		t.Fatalf("Uptime() = %v, want >= 50ms", got)
	}
}

func TestHealth_Snapshot(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	// Connect a client.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeConnection(w, r)
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	time.Sleep(50 * time.Millisecond)

	hs := hub.Health()
	if !hs.Alive {
		t.Error("Health.Alive should be true")
	}
	if !hs.Ready {
		t.Error("Health.Ready should be true")
	}
	if hs.State != "running" {
		t.Errorf("Health.State = %q, want %q", hs.State, "running")
	}
	if hs.Clients != 1 {
		t.Errorf("Health.Clients = %d, want 1", hs.Clients)
	}
	if hs.Uptime == 0 {
		t.Error("Health.Uptime should be > 0")
	}
}

// healthJSON is used to unmarshal the JSON response from health handlers.
type healthJSON struct {
	Alive    bool   `json:"alive"`
	Ready    bool   `json:"ready"`
	State    string `json:"state"`
	UptimeNs int64  `json:"uptime_ns"`
	Clients  int    `json:"clients"`
}

func TestHealthHandler_Running(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	time.Sleep(50 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	hub.HealthHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body healthJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.Alive {
		t.Error("alive should be true")
	}
	if !body.Ready {
		t.Error("ready should be true")
	}
	if body.State != "running" {
		t.Errorf("state = %q, want running", body.State)
	}
}

func TestHealthHandler_BeforeRun(t *testing.T) {
	hub := NewHub()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	hub.HealthHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var body healthJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Alive {
		t.Error("alive should be false")
	}
	if body.Ready {
		t.Error("ready should be false")
	}
}

func TestReadyHandler_Running(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	time.Sleep(50 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	hub.ReadyHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body healthJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.Ready {
		t.Error("ready should be true")
	}
}

func TestReadyHandler_Draining(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	time.Sleep(50 * time.Millisecond)

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer drainCancel()
	hub.Drain(drainCtx)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	hub.ReadyHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var body healthJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.Alive {
		t.Error("alive should be true while draining")
	}
	if body.Ready {
		t.Error("ready should be false while draining")
	}
	if body.State != "draining" {
		t.Errorf("state = %q, want draining", body.State)
	}
}

func TestUpgradeConnection_RejectsBeforeRun(t *testing.T) {
	hub := NewHub()
	// Intentionally do NOT call hub.Run().

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.UpgradeConnection(w, r)
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected dial to fail before Run(), but it succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}
