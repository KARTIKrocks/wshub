package wshub

import (
	"strings"
	"testing"
	"time"
)

func TestNewDebugMetrics(t *testing.T) {
	m := NewDebugMetrics()
	if m == nil {
		t.Fatal("NewDebugMetrics returned nil")
	}
	s := m.Stats()
	if s.ActiveConnections != 0 || s.TotalConnections != 0 || s.TotalMessages != 0 {
		t.Error("new DebugMetrics should have zero counters")
	}
}

func TestDebugMetricsConnections(t *testing.T) {
	m := NewDebugMetrics()
	m.IncrementConnections()
	m.IncrementConnections()
	m.DecrementConnections()

	s := m.Stats()
	if s.ActiveConnections != 1 {
		t.Errorf("ActiveConnections = %d, want 1", s.ActiveConnections)
	}
	if s.TotalConnections != 2 {
		t.Errorf("TotalConnections = %d, want 2", s.TotalConnections)
	}
}

func TestDebugMetricsMessages(t *testing.T) {
	m := NewDebugMetrics()
	m.IncrementMessages()
	m.IncrementMessages()
	m.RecordMessageSize(100)
	m.RecordMessageSize(200)

	s := m.Stats()
	if s.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2", s.TotalMessages)
	}
	if s.TotalMessageBytes != 300 {
		t.Errorf("TotalMessageBytes = %d, want 300", s.TotalMessageBytes)
	}
}

func TestDebugMetricsRooms(t *testing.T) {
	m := NewDebugMetrics()
	m.IncrementRoomJoins()
	m.IncrementRoomJoins()
	m.IncrementRoomLeaves()

	s := m.Stats()
	if s.TotalRoomJoins != 2 {
		t.Errorf("TotalRoomJoins = %d, want 2", s.TotalRoomJoins)
	}
	if s.TotalRoomLeaves != 1 {
		t.Errorf("TotalRoomLeaves = %d, want 1", s.TotalRoomLeaves)
	}
}

func TestDebugMetricsLatency(t *testing.T) {
	m := NewDebugMetrics()
	m.RecordLatency(10 * time.Millisecond)
	m.RecordLatency(20 * time.Millisecond)

	s := m.Stats()
	if s.AvgLatency != 15*time.Millisecond {
		t.Errorf("AvgLatency = %v, want 15ms", s.AvgLatency)
	}
}

func TestDebugMetricsErrors(t *testing.T) {
	m := NewDebugMetrics()
	m.IncrementErrors("read_error")
	m.IncrementErrors("read_error")
	m.IncrementErrors("write_error")

	s := m.Stats()
	if s.Errors["read_error"] != 2 {
		t.Errorf("read_error = %d, want 2", s.Errors["read_error"])
	}
	if s.Errors["write_error"] != 1 {
		t.Errorf("write_error = %d, want 1", s.Errors["write_error"])
	}
}

func TestDebugMetricsReset(t *testing.T) {
	m := NewDebugMetrics()
	m.IncrementConnections()
	m.IncrementMessages()
	m.RecordMessageSize(100)
	m.IncrementErrors("test")
	m.IncrementRoomJoins()
	m.RecordLatency(time.Millisecond)

	m.Reset()

	s := m.Stats()
	if s.ActiveConnections != 0 || s.TotalConnections != 0 || s.TotalMessages != 0 {
		t.Error("Reset should zero all counters")
	}
	if s.TotalMessageBytes != 0 || s.TotalRoomJoins != 0 || s.TotalRoomLeaves != 0 {
		t.Error("Reset should zero all counters")
	}
	if len(s.Errors) != 0 {
		t.Error("Reset should clear errors")
	}
	if s.AvgLatency != 0 {
		t.Error("Reset should clear latency")
	}
}

func TestDebugMetricsString(t *testing.T) {
	m := NewDebugMetrics()
	m.IncrementConnections()
	m.IncrementMessages()
	m.RecordMessageSize(100)
	m.IncrementErrors("test_error")

	s := m.String()
	if !strings.Contains(s, "wshub metrics") {
		t.Error("String should contain 'wshub metrics'")
	}
	if !strings.Contains(s, "connections") {
		t.Error("String should contain 'connections'")
	}
	if !strings.Contains(s, "test_error") {
		t.Error("String should contain error type")
	}
}

func TestDebugMetricsStatsUptime(t *testing.T) {
	m := NewDebugMetrics()
	time.Sleep(1100 * time.Millisecond)
	s := m.Stats()
	if s.Uptime < time.Second {
		t.Errorf("Uptime = %v, expected >= 1s", s.Uptime)
	}
}

func TestNoOpMetrics(t *testing.T) {
	// Just ensure NoOpMetrics implements the interface without panicking
	m := &NoOpMetrics{}
	m.IncrementConnections()
	m.DecrementConnections()
	m.IncrementMessages()
	m.RecordMessageSize(100)
	m.RecordLatency(time.Millisecond)
	m.IncrementErrors("test")
	m.IncrementRoomJoins()
	m.IncrementRoomLeaves()
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
