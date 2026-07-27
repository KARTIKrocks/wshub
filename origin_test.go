package wshub

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

// logRecord is a single captured log call, including its structured args so
// tests can assert on what the operator actually sees.
type logRecord struct {
	msg  string
	args []any
}

// field returns the value logged under key, slog-style.
func (r logRecord) field(key string) (any, bool) {
	for i := 0; i+1 < len(r.args); i += 2 {
		if k, ok := r.args[i].(string); ok && k == key {
			return r.args[i+1], true
		}
	}
	return nil, false
}

// stringField returns the value logged under key as a string.
func (r logRecord) stringField(t *testing.T, key string) string {
	t.Helper()
	v, ok := r.field(key)
	if !ok {
		t.Fatalf("log call %q has no %q field (args: %v)", r.msg, key, r.args)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("log field %q = %T, want string", key, v)
	}
	return s
}

// recordingLogger captures log calls so tests can assert on operator-facing
// diagnostics.
type recordingLogger struct {
	mu     sync.Mutex
	warns  []logRecord
	errors []logRecord
}

func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}

func (l *recordingLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, logRecord{msg: msg, args: args})
}

func (l *recordingLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, logRecord{msg: msg, args: args})
}

func (l *recordingLogger) warnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.warns)
}

func (l *recordingLogger) errorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors)
}

// lastWarn returns the most recent Warn call.
func (l *recordingLogger) lastWarn(t *testing.T) logRecord {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.warns) == 0 {
		t.Fatal("no Warn calls recorded")
	}
	return l.warns[len(l.warns)-1]
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

// The warning is the whole point of the change: it must actually name the
// rejected origin and host, and point at the fix.
func TestCheckOriginLogsActionableDetail(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	hub := NewHub(WithLogger(logger), WithMetrics(NewDebugMetrics()))

	hub.checkOrigin(originRequest("https://app.example.com", "ws.example.com"))

	rec := logger.lastWarn(t)
	if got := rec.stringField(t, "origin"); got != "https://app.example.com" {
		t.Errorf("origin field = %q, want the rejected origin", got)
	}
	if got := rec.stringField(t, "host"); got != "ws.example.com" {
		t.Errorf("host field = %q, want the request host", got)
	}
	if !strings.Contains(rec.msg, "WithCheckOrigin") {
		t.Errorf("message does not name the remedy: %q", rec.msg)
	}
}

// An Origin header is client-controlled; it must not be echoed into logs
// unbounded, and truncating it must not emit a broken rune.
func TestCheckOriginTruncatesLoggedOrigin(t *testing.T) {
	t.Parallel()

	// The truncation point must land mid-rune for this to test anything, so
	// each multi-byte case is padded to shift alignment. With no padding
	// "https://" (8 bytes) leaves 120 bytes, which divides evenly by both 2
	// and 4 and would never split a rune.
	var tests []struct {
		name   string
		origin string
	}
	add := func(name, origin string) {
		tests = append(tests, struct {
			name   string
			origin string
		}{name, origin})
	}

	add("ascii", "https://"+strings.Repeat("a", 4096))
	for pad := range 4 {
		prefix := "https://" + strings.Repeat("a", pad)
		add(fmt.Sprintf("2-byte rune, pad %d", pad), prefix+strings.Repeat("é", 4096))
		add(fmt.Sprintf("4-byte rune, pad %d", pad), prefix+strings.Repeat("🙂", 4096))
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &recordingLogger{}
			hub := NewHub(WithLogger(logger), WithMetrics(NewDebugMetrics()))

			if hub.checkOrigin(originRequest(tt.origin, "example.com")) {
				t.Fatal("oversized origin should be rejected")
			}

			logged := logger.lastWarn(t).stringField(t, "origin")

			if len(logged) > maxLoggedOriginLen+len("…") {
				t.Errorf("logged origin is %d bytes, want <= %d",
					len(logged), maxLoggedOriginLen+len("…"))
			}
			if len(logged) >= len(tt.origin) {
				t.Errorf("logged origin was not truncated (%d bytes)", len(logged))
			}
			if !utf8.ValidString(logged) {
				t.Errorf("logged origin is not valid UTF-8: %q", logged)
			}
		})
	}
}

// Regression test: the startup probe used a plausible-looking domain
// (attacker.example.com), so a checker matching a real suffix — the pattern
// the README documents — accepted the probe and was wrongly reported as
// accepting every origin.
func TestStartupProbeDoesNotFalselyWarn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		check func(*http.Request) bool
	}{
		{"suffix matcher", func(r *http.Request) bool {
			return strings.HasSuffix(r.Header.Get("Origin"), ".example.com")
		}},
		{"allowlist", AllowOrigins("https://app.example.com")},
		{"same origin", AllowSameOrigin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &recordingLogger{}
			NewHub(
				WithConfig(DefaultConfig().WithCheckOrigin(tt.check)),
				WithLogger(logger),
			)

			for _, rec := range logger.warns {
				if strings.Contains(rec.msg, "accepts every origin") {
					t.Errorf("restrictive checker wrongly warned: %q", rec.msg)
				}
			}
		})
	}
}

// Failed handshakes are reachable by any unauthenticated client, so none of
// the handshake log sites may emit a line per attempt.
func TestRejectedUpgradesDoNotFloodLogs(t *testing.T) {
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

	const attempts = 20
	for range attempts {
		conn, _, err := dialer.Dial(wsURL, http.Header{
			"Origin": []string{"https://attacker.example"},
		})
		if err == nil {
			conn.Close()
			t.Fatal("cross-origin dial should have been rejected")
		}
	}

	// One rate-limited warning, one rate-limited error — not one per attempt.
	if got := logger.warnCount(); got != 1 {
		t.Errorf("warn count = %d, want 1 for %d rejections", got, attempts)
	}
	if got := logger.errorCount(); got > 1 {
		t.Errorf("error count = %d for %d rejections — logs are floodable",
			got, attempts)
	}

	// Metrics still carry the exact count.
	if got := metrics.Stats().Errors["origin_rejected"]; got != attempts {
		t.Errorf("origin_rejected = %d, want %d", got, attempts)
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
