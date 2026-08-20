package wshub

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector is an interface for collecting metrics.
// Applications can implement this with Prometheus, StatsD, etc.
type MetricsCollector interface {
	// Connections
	IncrementConnections()
	DecrementConnections()

	// Messages
	IncrementMessagesReceived()
	IncrementMessagesSent(count int)
	IncrementMessagesDropped()

	// Observations
	RecordMessageSize(size int)
	RecordLatency(duration time.Duration)
	RecordBroadcastDuration(duration time.Duration)

	// Rooms
	IncrementRoomJoins()
	IncrementRoomLeaves()
	IncrementRooms()
	DecrementRooms()

	// Errors
	IncrementErrors(errorType string)
}

// NoOpMetrics is a default implementation that does nothing.
type NoOpMetrics struct{}

func (n *NoOpMetrics) IncrementConnections()                          {}
func (n *NoOpMetrics) DecrementConnections()                          {}
func (n *NoOpMetrics) IncrementMessagesReceived()                     {}
func (n *NoOpMetrics) IncrementMessagesSent(count int)                {}
func (n *NoOpMetrics) IncrementMessagesDropped()                      {}
func (n *NoOpMetrics) RecordMessageSize(size int)                     {}
func (n *NoOpMetrics) RecordLatency(duration time.Duration)           {}
func (n *NoOpMetrics) RecordBroadcastDuration(duration time.Duration) {}
func (n *NoOpMetrics) IncrementErrors(errorType string)               {}
func (n *NoOpMetrics) IncrementRoomJoins()                            {}
func (n *NoOpMetrics) IncrementRoomLeaves()                           {}
func (n *NoOpMetrics) IncrementRooms()                                {}
func (n *NoOpMetrics) DecrementRooms()                                {}

// DebugStats is a point-in-time snapshot returned by DebugMetrics.Stats().
type DebugStats struct {
	ActiveConnections int64
	TotalConnections  int64
	TotalMessagesRecv int64
	TotalMessagesSent int64
	TotalDropped      int64
	TotalMessageBytes int64
	TotalRoomJoins    int64
	TotalRoomLeaves   int64
	ActiveRooms       int64
	AvgLatency        time.Duration
	AvgBroadcast      time.Duration
	Errors            map[string]int64
	Uptime            time.Duration
}

// DebugMetrics is a thread-safe in-memory MetricsCollector for development
// and testing. Use Stats() to read a snapshot or String() to print a summary.
//
// Usage:
//
//	m := wshub.NewDebugMetrics()
//	hub := wshub.NewHub(wshub.WithMetrics(m))
//	...
//	fmt.Println(m)           // pretty-print summary
//	stats := m.Stats()       // programmatic access
type DebugMetrics struct {
	activeConnections atomic.Int64
	totalConnections  atomic.Int64
	totalMessagesRecv atomic.Int64
	totalMessagesSent atomic.Int64
	totalDropped      atomic.Int64
	totalMessageBytes atomic.Int64
	totalRoomJoins    atomic.Int64
	totalRoomLeaves   atomic.Int64
	activeRooms       atomic.Int64

	latencyMu    sync.Mutex
	latencyTotal int64 // nanoseconds, protected by latencyMu
	latencyCount int64 // protected by latencyMu

	broadcastMu    sync.Mutex
	broadcastTotal int64 // nanoseconds, protected by broadcastMu
	broadcastCount int64 // protected by broadcastMu

	errorsMu sync.RWMutex
	errors   map[string]int64

	startMu   sync.RWMutex
	startTime time.Time
}

// NewDebugMetrics creates a new DebugMetrics instance.
func NewDebugMetrics() *DebugMetrics {
	return &DebugMetrics{
		errors:    make(map[string]int64),
		startTime: time.Now(),
	}
}

func (d *DebugMetrics) IncrementConnections() {
	d.activeConnections.Add(1)
	d.totalConnections.Add(1)
}

func (d *DebugMetrics) DecrementConnections() {
	d.activeConnections.Add(-1)
}

func (d *DebugMetrics) IncrementMessagesReceived() {
	d.totalMessagesRecv.Add(1)
}

func (d *DebugMetrics) IncrementMessagesSent(count int) {
	d.totalMessagesSent.Add(int64(count))
}

func (d *DebugMetrics) IncrementMessagesDropped() {
	d.totalDropped.Add(1)
}

func (d *DebugMetrics) RecordMessageSize(size int) {
	d.totalMessageBytes.Add(int64(size))
}

func (d *DebugMetrics) RecordLatency(duration time.Duration) {
	d.latencyMu.Lock()
	d.latencyTotal += int64(duration)
	d.latencyCount++
	d.latencyMu.Unlock()
}

func (d *DebugMetrics) RecordBroadcastDuration(duration time.Duration) {
	d.broadcastMu.Lock()
	d.broadcastTotal += int64(duration)
	d.broadcastCount++
	d.broadcastMu.Unlock()
}

func (d *DebugMetrics) IncrementErrors(errorType string) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()
	d.errors[errorType]++
}

func (d *DebugMetrics) IncrementRoomJoins() {
	d.totalRoomJoins.Add(1)
}

func (d *DebugMetrics) IncrementRoomLeaves() {
	d.totalRoomLeaves.Add(1)
}

func (d *DebugMetrics) IncrementRooms() {
	d.activeRooms.Add(1)
}

func (d *DebugMetrics) DecrementRooms() {
	d.activeRooms.Add(-1)
}

// Stats returns a point-in-time snapshot of all metrics.
func (d *DebugMetrics) Stats() DebugStats {
	d.errorsMu.RLock()
	errCopy := make(map[string]int64, len(d.errors))
	maps.Copy(errCopy, d.errors)
	d.errorsMu.RUnlock()

	d.latencyMu.Lock()
	var avgLatency time.Duration
	if d.latencyCount > 0 {
		avgLatency = time.Duration(d.latencyTotal / d.latencyCount)
	}
	d.latencyMu.Unlock()

	d.broadcastMu.Lock()
	var avgBroadcast time.Duration
	if d.broadcastCount > 0 {
		avgBroadcast = time.Duration(d.broadcastTotal / d.broadcastCount)
	}
	d.broadcastMu.Unlock()

	d.startMu.RLock()
	uptime := time.Since(d.startTime).Round(time.Second)
	d.startMu.RUnlock()

	return DebugStats{
		ActiveConnections: d.activeConnections.Load(),
		TotalConnections:  d.totalConnections.Load(),
		TotalMessagesRecv: d.totalMessagesRecv.Load(),
		TotalMessagesSent: d.totalMessagesSent.Load(),
		TotalDropped:      d.totalDropped.Load(),
		TotalMessageBytes: d.totalMessageBytes.Load(),
		TotalRoomJoins:    d.totalRoomJoins.Load(),
		TotalRoomLeaves:   d.totalRoomLeaves.Load(),
		ActiveRooms:       d.activeRooms.Load(),
		AvgLatency:        avgLatency,
		AvgBroadcast:      avgBroadcast,
		Errors:            errCopy,
		Uptime:            uptime,
	}
}

// Reset zeroes all counters and resets the uptime clock.
func (d *DebugMetrics) Reset() {
	d.activeConnections.Store(0)
	d.totalConnections.Store(0)
	d.totalMessagesRecv.Store(0)
	d.totalMessagesSent.Store(0)
	d.totalDropped.Store(0)
	d.totalMessageBytes.Store(0)
	d.totalRoomJoins.Store(0)
	d.totalRoomLeaves.Store(0)
	d.activeRooms.Store(0)

	d.latencyMu.Lock()
	d.latencyTotal = 0
	d.latencyCount = 0
	d.latencyMu.Unlock()

	d.broadcastMu.Lock()
	d.broadcastTotal = 0
	d.broadcastCount = 0
	d.broadcastMu.Unlock()

	d.errorsMu.Lock()
	d.errors = make(map[string]int64)
	d.errorsMu.Unlock()

	d.startMu.Lock()
	d.startTime = time.Now()
	d.startMu.Unlock()
}

// String returns a human-readable summary of all metrics.
// Implements fmt.Stringer so it prints naturally with fmt.Println(m).
func (d *DebugMetrics) String() string {
	s := d.Stats()

	var sb strings.Builder
	fmt.Fprintf(&sb, "wshub metrics (uptime: %s)\n", s.Uptime)
	fmt.Fprintf(&sb, "  connections : %d active, %d total\n", s.ActiveConnections, s.TotalConnections)
	fmt.Fprintf(&sb, "  messages    : %d recv, %d sent, %d dropped, %s\n",
		s.TotalMessagesRecv, s.TotalMessagesSent, s.TotalDropped, formatBytes(s.TotalMessageBytes))
	fmt.Fprintf(&sb, "  rooms       : %d active, %d joins, %d leaves\n", s.ActiveRooms, s.TotalRoomJoins, s.TotalRoomLeaves)

	if s.AvgLatency > 0 {
		fmt.Fprintf(&sb, "  avg latency : %s\n", s.AvgLatency)
	}

	if s.AvgBroadcast > 0 {
		fmt.Fprintf(&sb, "  avg bcast   : %s\n", s.AvgBroadcast)
	}

	if len(s.Errors) > 0 {
		// Sort error types for deterministic output.
		keys := make([]string, 0, len(s.Errors))
		for k := range s.Errors {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", k, s.Errors[k]))
		}
		fmt.Fprintf(&sb, "  errors      : %s\n", strings.Join(parts, " "))
	}

	return sb.String()
}

// formatBytes converts a byte count to a human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
