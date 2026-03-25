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
	a := &Adapter{subject: defaultSubject}
	WithSubject("custom.subject")(a)
	if a.subject != "custom.subject" {
		t.Errorf("subject = %q, want %q", a.subject, "custom.subject")
	}
}

func TestWithSubjectEmpty(t *testing.T) {
	a := &Adapter{subject: defaultSubject}
	WithSubject("")(a)
	if a.subject != defaultSubject {
		t.Errorf("subject = %q, want default %q", a.subject, defaultSubject)
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
