package redis

import (
	"testing"

	"github.com/KARTIKrocks/wshub"
)

func TestAdapterImplementsInterface(t *testing.T) {
	// Compile-time check that *Adapter satisfies wshub.Adapter.
	var _ wshub.Adapter = (*Adapter)(nil)
}

func TestWithChannel(t *testing.T) {
	a := &Adapter{channel: defaultChannel}
	WithChannel("custom:chan")(a)
	if a.channel != "custom:chan" {
		t.Errorf("channel = %q, want %q", a.channel, "custom:chan")
	}
}

func TestWithChannelEmpty(t *testing.T) {
	a := &Adapter{channel: defaultChannel}
	WithChannel("")(a)
	if a.channel != defaultChannel {
		t.Errorf("channel = %q, want default %q", a.channel, defaultChannel)
	}
}

func TestCloseIdempotent(t *testing.T) {
	a := New(nil) // nil client is fine — Close doesn't touch it.
	if err := a.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestNewDefaults(t *testing.T) {
	// Verify New sets correct defaults without panicking.
	// A real integration test requires a running Redis instance.
	a := New(nil) // nil client is fine for construction.
	if a.channel != defaultChannel {
		t.Errorf("channel = %q, want %q", a.channel, defaultChannel)
	}
	if a.closed {
		t.Error("should not be closed initially")
	}
}
