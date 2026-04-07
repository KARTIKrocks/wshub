package prometheus

import (
	"testing"
	"time"

	"github.com/KARTIKrocks/wshub"
	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestCollector(t *testing.T) (*Collector, *prom.Registry) {
	t.Helper()
	reg := prom.NewRegistry()
	c := New(WithRegistry(reg))
	return c, reg
}

func TestCollectorImplementsInterface(t *testing.T) {
	reg := prom.NewRegistry()
	var _ wshub.MetricsCollector = New(WithRegistry(reg))
}

func TestConnections(t *testing.T) {
	c, reg := newTestCollector(t)
	c.IncrementConnections()
	c.IncrementConnections()
	c.DecrementConnections()

	assertGauge(t, reg, "wshub_connections_active", 1)
	assertCounter(t, reg, "wshub_connections_total", 2)
}

func TestMessagesReceived(t *testing.T) {
	c, reg := newTestCollector(t)
	c.IncrementMessagesReceived()
	c.IncrementMessagesReceived()

	assertCounter(t, reg, "wshub_messages_received_total", 2)
}

func TestMessagesSent(t *testing.T) {
	c, reg := newTestCollector(t)
	c.IncrementMessagesSent(1)
	c.IncrementMessagesSent(5)

	assertCounter(t, reg, "wshub_messages_sent_total", 6)
}

func TestMessagesDropped(t *testing.T) {
	c, reg := newTestCollector(t)
	c.IncrementMessagesDropped()
	c.IncrementMessagesDropped()

	assertCounter(t, reg, "wshub_messages_dropped_total", 2)
}

func TestMessageSize(t *testing.T) {
	c, reg := newTestCollector(t)
	c.RecordMessageSize(100)
	c.RecordMessageSize(200)

	assertCounter(t, reg, "wshub_message_received_bytes_total", 300)
}

func TestLatency(t *testing.T) {
	c, reg := newTestCollector(t)
	c.RecordLatency(10 * time.Millisecond)
	c.RecordLatency(20 * time.Millisecond)

	assertHistogramCount(t, reg, "wshub_message_latency_seconds", 2)
}

func TestBroadcastDuration(t *testing.T) {
	c, reg := newTestCollector(t)
	c.RecordBroadcastDuration(100 * time.Microsecond)
	c.RecordBroadcastDuration(200 * time.Microsecond)

	assertHistogramCount(t, reg, "wshub_broadcast_duration_seconds", 2)
}

func TestRooms(t *testing.T) {
	c, reg := newTestCollector(t)
	c.IncrementRooms()
	c.IncrementRooms()
	c.DecrementRooms()
	c.IncrementRoomJoins()
	c.IncrementRoomJoins()
	c.IncrementRoomLeaves()

	assertGauge(t, reg, "wshub_rooms_active", 1)
	assertCounter(t, reg, "wshub_room_joins_total", 2)
	assertCounter(t, reg, "wshub_room_leaves_total", 1)
}

func TestErrors(t *testing.T) {
	c, reg := newTestCollector(t)
	c.IncrementErrors("write_error")
	c.IncrementErrors("write_error")
	c.IncrementErrors("read_error")

	families := gatherOrFail(t, reg)
	mf := findFamily(families, "wshub_errors_total")
	if mf == nil {
		t.Fatal("metric wshub_errors_total not found")
	}

	var writeCount, readCount float64
	for _, m := range mf.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "type" {
				switch l.GetValue() {
				case "write_error":
					writeCount = m.GetCounter().GetValue()
				case "read_error":
					readCount = m.GetCounter().GetValue()
				}
			}
		}
	}
	if writeCount != 2 {
		t.Errorf("write_error = %v, want 2", writeCount)
	}
	if readCount != 1 {
		t.Errorf("read_error = %v, want 1", readCount)
	}
}

func TestCustomNamespace(t *testing.T) {
	reg := prom.NewRegistry()
	c := New(WithRegistry(reg), WithNamespace("myapp"))
	c.IncrementConnections()

	assertGauge(t, reg, "myapp_connections_active", 1)
	assertCounter(t, reg, "myapp_connections_total", 1)
}

func TestCustomBuckets(t *testing.T) {
	reg := prom.NewRegistry()
	c := New(
		WithRegistry(reg),
		WithLatencyBuckets([]float64{0.01, 0.1, 1}),
		WithBroadcastBuckets([]float64{0.001, 0.01}),
	)
	c.RecordLatency(50 * time.Millisecond)
	c.RecordBroadcastDuration(5 * time.Millisecond)

	assertHistogramCount(t, reg, "wshub_message_latency_seconds", 1)
	assertHistogramCount(t, reg, "wshub_broadcast_duration_seconds", 1)
}

// --- helpers ---

func gatherOrFail(t *testing.T, reg *prom.Registry) []*dto.MetricFamily {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	return families
}

func findFamily(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, mf := range families {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func assertGauge(t *testing.T, reg *prom.Registry, name string, want float64) {
	t.Helper()
	families := gatherOrFail(t, reg)
	mf := findFamily(families, name)
	if mf == nil {
		t.Fatalf("metric %s not found", name)
	}
	got := mf.GetMetric()[0].GetGauge().GetValue()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func assertCounter(t *testing.T, reg *prom.Registry, name string, want float64) {
	t.Helper()
	families := gatherOrFail(t, reg)
	mf := findFamily(families, name)
	if mf == nil {
		t.Fatalf("metric %s not found", name)
	}
	got := mf.GetMetric()[0].GetCounter().GetValue()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func assertHistogramCount(t *testing.T, reg *prom.Registry, name string, want uint64) {
	t.Helper()
	families := gatherOrFail(t, reg)
	mf := findFamily(families, name)
	if mf == nil {
		t.Fatalf("metric %s not found", name)
	}
	got := mf.GetMetric()[0].GetHistogram().GetSampleCount()
	if got != want {
		t.Errorf("%s count = %v, want %v", name, got, want)
	}
}
