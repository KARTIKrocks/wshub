package wshub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()

	if c.ReadBufferSize != 1024 {
		t.Errorf("ReadBufferSize = %d, want 1024", c.ReadBufferSize)
	}
	if c.WriteBufferSize != 1024 {
		t.Errorf("WriteBufferSize = %d, want 1024", c.WriteBufferSize)
	}
	if c.WriteWait != 10*time.Second {
		t.Errorf("WriteWait = %v, want 10s", c.WriteWait)
	}
	if c.PongWait != 60*time.Second {
		t.Errorf("PongWait = %v, want 60s", c.PongWait)
	}
	if c.PingPeriod != 54*time.Second {
		t.Errorf("PingPeriod = %v, want 54s", c.PingPeriod)
	}
	if c.MaxMessageSize != 512*1024 {
		t.Errorf("MaxMessageSize = %d, want %d", c.MaxMessageSize, 512*1024)
	}
	if c.SendChannelSize != 256 {
		t.Errorf("SendChannelSize = %d, want 256", c.SendChannelSize)
	}
	if c.EnableCompression != false {
		t.Error("EnableCompression should be false")
	}
	if c.CheckOrigin == nil {
		t.Error("CheckOrigin should not be nil")
	}
}

func TestApplyConfigDefaults(t *testing.T) {
	// Partial config should get defaults for unset fields
	c := applyConfigDefaults(Config{ReadBufferSize: 4096})

	if c.ReadBufferSize != 4096 {
		t.Errorf("ReadBufferSize = %d, want 4096", c.ReadBufferSize)
	}
	if c.WriteBufferSize != 1024 {
		t.Errorf("WriteBufferSize = %d, want 1024 (default)", c.WriteBufferSize)
	}
	if c.WriteWait != 10*time.Second {
		t.Errorf("WriteWait = %v, want 10s (default)", c.WriteWait)
	}
	if c.CheckOrigin == nil {
		t.Error("CheckOrigin should default to non-nil")
	}
}

func TestConfigBuilderMethods(t *testing.T) {
	c := DefaultConfig().
		WithBufferSizes(2048, 4096).
		WithTimeouts(5*time.Second, 30*time.Second, 25*time.Second).
		WithMaxMessageSize(1024*1024).
		WithSendChannelSize(512).
		WithCompression(true).
		WithSubprotocols("chat", "binary")

	if c.ReadBufferSize != 2048 {
		t.Errorf("ReadBufferSize = %d, want 2048", c.ReadBufferSize)
	}
	if c.WriteBufferSize != 4096 {
		t.Errorf("WriteBufferSize = %d, want 4096", c.WriteBufferSize)
	}
	if c.WriteWait != 5*time.Second {
		t.Errorf("WriteWait = %v, want 5s", c.WriteWait)
	}
	if c.PongWait != 30*time.Second {
		t.Errorf("PongWait = %v, want 30s", c.PongWait)
	}
	if c.PingPeriod != 25*time.Second {
		t.Errorf("PingPeriod = %v, want 25s", c.PingPeriod)
	}
	if c.MaxMessageSize != 1024*1024 {
		t.Errorf("MaxMessageSize = %d, want 1MB", c.MaxMessageSize)
	}
	if c.SendChannelSize != 512 {
		t.Errorf("SendChannelSize = %d, want 512", c.SendChannelSize)
	}
	if !c.EnableCompression {
		t.Error("EnableCompression should be true")
	}
	if len(c.Subprotocols) != 2 || c.Subprotocols[0] != "chat" {
		t.Errorf("Subprotocols = %v, want [chat binary]", c.Subprotocols)
	}
}

func TestAllowAllOrigins(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "https://evil.com")
	if !AllowAllOrigins(r) {
		t.Error("AllowAllOrigins should allow all origins")
	}
}

func TestAllowSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"empty origin", "", "localhost:8080", true},
		{"same http", "http://localhost:8080", "localhost:8080", true},
		{"same https", "https://example.com", "example.com", true},
		{"different", "https://evil.com", "example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ws", nil)
			r.Host = tt.host
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := AllowSameOrigin(r); got != tt.want {
				t.Errorf("AllowSameOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllowOrigins(t *testing.T) {
	check := AllowOrigins("https://example.com", "https://app.example.com")

	tests := []struct {
		origin string
		want   bool
	}{
		{"", true},
		{"https://example.com", true},
		{"https://app.example.com", true},
		{"https://evil.com", false},
	}
	for _, tt := range tests {
		r := &http.Request{Header: http.Header{}}
		if tt.origin != "" {
			r.Header.Set("Origin", tt.origin)
		}
		if got := check(r); got != tt.want {
			t.Errorf("AllowOrigins(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestWithCheckOrigin(t *testing.T) {
	called := false
	c := DefaultConfig().WithCheckOrigin(func(r *http.Request) bool {
		called = true
		return true
	})
	r := httptest.NewRequest("GET", "/ws", nil)
	c.CheckOrigin(r)
	if !called {
		t.Error("custom CheckOrigin was not called")
	}
}

func TestValidateConfig_SmallBuffers(t *testing.T) {
	c := Config{ReadBufferSize: 64, WriteBufferSize: 64}
	warnings := validateConfig(c)
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
	for _, w := range warnings {
		if !strings.Contains(w, "very small") {
			t.Errorf("unexpected warning: %s", w)
		}
	}
}

func TestValidateConfig_NoWarnings(t *testing.T) {
	c := DefaultConfig()
	warnings := validateConfig(c)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateConfig_OnlyReadSmall(t *testing.T) {
	c := Config{ReadBufferSize: 100, WriteBufferSize: 1024}
	warnings := validateConfig(c)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestApplyConfigDefaults_PingPeriodClamp(t *testing.T) {
	// PingPeriod >= PongWait should be clamped to 90% of PongWait.
	c := applyConfigDefaults(Config{
		PongWait:   10 * time.Second,
		PingPeriod: 15 * time.Second, // larger than PongWait
	})
	expected := (10 * time.Second * 9) / 10
	if c.PingPeriod != expected {
		t.Errorf("PingPeriod = %v, want %v (90%% of PongWait)", c.PingPeriod, expected)
	}
}

func TestAllowSameOrigin_InvalidURL(t *testing.T) {
	r := httptest.NewRequest("GET", "/ws", nil)
	r.Header.Set("Origin", "://invalid-url")
	if AllowSameOrigin(r) {
		t.Error("invalid URL origin should be rejected")
	}
}
