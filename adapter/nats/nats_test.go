package nats

import (
	"testing"

	"github.com/KARTIKrocks/wshub"
)

func TestAdapterImplementsInterface(t *testing.T) {
	// Compile-time check that *Adapter satisfies wshub.Adapter.
	var _ wshub.Adapter = (*Adapter)(nil)
}

func TestWithSubject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "custom subject",
			input:    "custom.subject",
			expected: "custom.subject",
		},
		{
			name:     "empty subject uses default",
			input:    "",
			expected: defaultSubject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{subject: defaultSubject}
			WithSubject(tt.input)(a)
			if a.subject != tt.expected {
				t.Errorf("subject = %q, want %q", a.subject, tt.expected)
			}
		})
	}
}

func TestNewDefaults(t *testing.T) {
	a := New(nil)
	if a.subject != defaultSubject {
		t.Errorf("subject = %q, want %q", a.subject, defaultSubject)
	}
	if a.closed {
		t.Error("should not be closed initially")
	}
}

func TestCloseIdempotent(t *testing.T) {
	a := New(nil)
	if err := a.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
