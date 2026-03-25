package wshub

import (
	"errors"
	"testing"
)

func TestIsChanSendPanic_Nil(t *testing.T) {
	if isChanSendPanic(nil) {
		t.Error("nil should not be a chan send panic")
	}
}

func TestIsChanSendPanic_ErrorType(t *testing.T) {
	err := errors.New("send on closed channel")
	if !isChanSendPanic(err) {
		t.Error("error containing 'send on closed channel' should be detected")
	}
}

func TestIsChanSendPanic_ErrorTypeNoMatch(t *testing.T) {
	err := errors.New("something else")
	if isChanSendPanic(err) {
		t.Error("unrelated error should not match")
	}
}

func TestIsChanSendPanic_StringType(t *testing.T) {
	if !isChanSendPanic("send on closed channel") {
		t.Error("string containing 'send on closed channel' should be detected")
	}
}

func TestIsChanSendPanic_StringTypeNoMatch(t *testing.T) {
	if isChanSendPanic("index out of range") {
		t.Error("unrelated string should not match")
	}
}

func TestIsChanSendPanic_IntType(t *testing.T) {
	if isChanSendPanic(42) {
		t.Error("int type should return false (default case)")
	}
}
