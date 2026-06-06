package wshub

import (
	"errors"
	"testing"
)

func TestIsChanSendPanic(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{
			name:  "nil",
			input: nil,
			want:  false,
		},
		{
			name:  "error type with send on closed channel",
			input: errors.New("send on closed channel"),
			want:  true,
		},
		{
			name:  "error type no match",
			input: errors.New("something else"),
			want:  false,
		},
		{
			name:  "string type with send on closed channel",
			input: "send on closed channel",
			want:  true,
		},
		{
			name:  "string type no match",
			input: "index out of range",
			want:  false,
		},
		{
			name:  "int type (default case)",
			input: 42,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isChanSendPanic(tt.input); got != tt.want {
				t.Errorf("isChanSendPanic(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
