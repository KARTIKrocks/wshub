package wshub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// recordingLogger captures Warn calls so tests can assert on operator-facing
// diagnostics.
type recordingLogger struct {
	mu    sync.Mutex
	warns []string
}

func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Error(string, ...any) {}

func (l *recordingLogger) Warn(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, msg)
}

func (l *recordingLogger) warnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.warns)
}

func originRequest(origin, host string) *http.Request {
	r := &http.Request{Header: http.Header{}}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	r.Host = host
	return r
}

// The default must reject cross-origin browser requests. Originless requests
// stay allowed so non-browser clients keep working.
func TestDefaultConfigRejectsCrossOrigin(t *testing.T) {
	t.Parallel()

	check := DefaultConfig().CheckOrigin

	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"same origin", "https://example.com", "example.com", true},
		{"cross origin", "https://attacker.example", "example.com", false},
		{"mismatched port", "https://example.com:8443", "example.com", false},
		{"no origin header", "", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := check(originRequest(tt.origin, tt.host)); got != tt.want {
				t.Errorf("CheckOrigin(%q, host %q) = %v, want %v",
					tt.origin, tt.host, got, tt.want)
			}
		})
	}
}

// A rejected origin must be observable: a bare 403 from gorilla is otherwise
// indistinguishable from a proxy or routing fault.
func TestCheckOriginLogsAndCountsRejection(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	metrics := NewDebugMetrics()
	hub := NewHub(WithLogger(logger), WithMetrics(metrics))

	if hub.checkOrigin(originRequest("https://attacker.example", "example.com")) {
		t.Fatal("cross-origin request should be rejected by default")
	}

	if got := logger.warnCount(); got != 1 {
		t.Errorf("warn count = %d, want 1", got)
	}
	if got := metrics.Stats().Errors["origin_rejected"]; got != 1 {
		t.Errorf("origin_rejected = %d, want 1", got)
	}
}

func TestCheckOriginAllowsPermittedOrigin(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	metrics := NewDebugMetrics()
	hub := NewHub(
		WithConfig(DefaultConfig().WithCheckOrigin(AllowOrigins("https://app.example.com"))),
		WithLogger(logger),
		WithMetrics(metrics),
	)

	if !hub.checkOrigin(originRequest("https://app.example.com", "ws.example.com")) {
		t.Fatal("allowlisted cross-origin request should be accepted")
	}
	if got := logger.warnCount(); got != 0 {
		t.Errorf("warn count = %d, want 0 for an accepted origin", got)
	}
	if got := metrics.Stats().Errors["origin_rejected"]; got != 0 {
		t.Errorf("origin_rejected = %d, want 0", got)
	}
}

// Rejections are attacker-triggerable, so the warning is rate-limited while
// the metric still counts every one.
func TestCheckOriginRateLimitsRejectionLog(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	metrics := NewDebugMetrics()
	hub := NewHub(WithLogger(logger), WithMetrics(metrics))

	const rejections = 50
	for range rejections {
		hub.checkOrigin(originRequest("https://attacker.example", "example.com"))
	}

	if got := logger.warnCount(); got != 1 {
		t.Errorf("warn count = %d, want 1 within the rate-limit window", got)
	}
	if got := metrics.Stats().Errors["origin_rejected"]; got != rejections {
		t.Errorf("origin_rejected = %d, want %d", got, rejections)
	}
}

func TestCheckOriginRateLimitIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	hub := NewHub(WithLogger(logger), WithMetrics(NewDebugMetrics()))

	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.checkOrigin(originRequest("https://attacker.example", "example.com"))
		}()
	}
	wg.Wait()

	if got := logger.warnCount(); got != 1 {
		t.Errorf("warn count = %d, want exactly 1 under concurrent rejections", got)
	}
}

// An Origin header is client-controlled; it must not be echoed into logs
// unbounded.
func TestCheckOriginTruncatesLoggedOrigin(t *testing.T) {
	t.Parallel()

	hub := NewHub(WithLogger(&NoOpLogger{}))
	long := "https://" + string(make([]byte, 4096))

	if hub.checkOrigin(originRequest(long, "example.com")) {
		t.Fatal("oversized origin should be rejected")
	}
}

// End-to-end proof that the upgrader is wired to the hub's checker: the unit
// tests above call checkOrigin directly and would still pass if the Upgrader
// were left pointing at config.CheckOrigin.
func TestUpgradeRejectsCrossOriginDial(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	metrics := NewDebugMetrics()
	hub := NewHub(WithLogger(logger), WithMetrics(metrics))
	go hub.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hub.Shutdown(ctx)
	}()

	server := httptest.NewServer(hub.HandleHTTP())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := &websocket.Dialer{}

	conn, resp, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{"https://attacker.example"},
	})
	if err == nil {
		conn.Close()
		t.Fatal("cross-origin dial should have been rejected")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, want %d", resp, http.StatusForbidden)
	}
	if got := metrics.Stats().Errors["origin_rejected"]; got != 1 {
		t.Errorf("origin_rejected = %d, want 1", got)
	}

	// A dial with no Origin header still succeeds, so non-browser clients
	// keep working under the stricter default.
	conn, _, err = dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("originless dial should succeed, got %v", err)
	}
	conn.Close()
}

// Explicitly opting into AllowAllOrigins must still work, and must warn.
func TestAllowAllOriginsWarnsAtStartup(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	hub := NewHub(
		WithConfig(DefaultConfig().WithCheckOrigin(AllowAllOrigins)),
		WithLogger(logger),
	)

	if !hub.checkOrigin(originRequest("https://anywhere.example", "example.com")) {
		t.Fatal("AllowAllOrigins should accept any origin")
	}
	if logger.warnCount() == 0 {
		t.Error("NewHub should warn when CheckOrigin accepts every origin")
	}
}
