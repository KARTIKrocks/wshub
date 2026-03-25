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
	IncrementConnections()
	DecrementConnections()
	IncrementMessages()
	RecordMessageSize(size int)
	RecordLatency(duration time.Duration)
	IncrementErrors(errorType string)
	IncrementRoomJoins()
	IncrementRoomLeaves()
}

// NoOpMetrics is a default implementation that does nothing.
type NoOpMetrics struct{}

func (n *NoOpMetrics) IncrementConnections()                {}
func (n *NoOpMetrics) DecrementConnections()                {}
func (n *NoOpMetrics) IncrementMessages()                   {}
func (n *NoOpMetrics) RecordMessageSize(size int)           {}
func (n *NoOpMetrics) RecordLatency(duration time.Duration) {}
func (n *NoOpMetrics) IncrementErrors(errorType string)     {}
func (n *NoOpMetrics) IncrementRoomJoins()                  {}
func (n *NoOpMetrics) IncrementRoomLeaves()                 {}

// DebugStats is a point-in-time snapshot returned by DebugMetrics.Stats().
type DebugStats struct {
	ActiveConnections int64
	TotalConnections  int64
	TotalMessages     int64
	TotalMessageBytes int64
	TotalRoomJoins    int64
	TotalRoomLeaves   int64
	AvgLatency        time.Duration
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
	activeConnections int64 // atomic
	totalConnections  int64 // atomic
	totalMessages     int64 // atomic
	totalMessageBytes int64 // atomic
	totalRoomJoins    int64 // atomic
	totalRoomLeaves   int64 // atomic
	latencyMu         sync.Mutex
	latencyTotal      int64 // nanoseconds, protected by latencyMu
	latencyCount      int64 // protected by latencyMu

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
	atomic.AddInt64(&d.activeConnections, 1)
	atomic.AddInt64(&d.totalConnections, 1)
}

func (d *DebugMetrics) DecrementConnections() {
	atomic.AddInt64(&d.activeConnections, -1)
}

func (d *DebugMetrics) IncrementMessages() {
	atomic.AddInt64(&d.totalMessages, 1)
}

func (d *DebugMetrics) RecordMessageSize(size int) {
	atomic.AddInt64(&d.totalMessageBytes, int64(size))
}

func (d *DebugMetrics) RecordLatency(duration time.Duration) {
	d.latencyMu.Lock()
	d.latencyTotal += int64(duration)
	d.latencyCount++
	d.latencyMu.Unlock()
}

func (d *DebugMetrics) IncrementErrors(errorType string) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()
	d.errors[errorType]++
}

func (d *DebugMetrics) IncrementRoomJoins() {
	atomic.AddInt64(&d.totalRoomJoins, 1)
}

func (d *DebugMetrics) IncrementRoomLeaves() {
	atomic.AddInt64(&d.totalRoomLeaves, 1)
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

	d.startMu.RLock()
	uptime := time.Since(d.startTime).Round(time.Second)
	d.startMu.RUnlock()

	return DebugStats{
		ActiveConnections: atomic.LoadInt64(&d.activeConnections),
		TotalConnections:  atomic.LoadInt64(&d.totalConnections),
		TotalMessages:     atomic.LoadInt64(&d.totalMessages),
		TotalMessageBytes: atomic.LoadInt64(&d.totalMessageBytes),
		TotalRoomJoins:    atomic.LoadInt64(&d.totalRoomJoins),
		TotalRoomLeaves:   atomic.LoadInt64(&d.totalRoomLeaves),
		AvgLatency:        avgLatency,
		Errors:            errCopy,
		Uptime:            uptime,
	}
}

// Reset zeroes all counters and resets the uptime clock.
func (d *DebugMetrics) Reset() {
	atomic.StoreInt64(&d.activeConnections, 0)
	atomic.StoreInt64(&d.totalConnections, 0)
	atomic.StoreInt64(&d.totalMessages, 0)
	atomic.StoreInt64(&d.totalMessageBytes, 0)
	atomic.StoreInt64(&d.totalRoomJoins, 0)
	atomic.StoreInt64(&d.totalRoomLeaves, 0)
	d.latencyMu.Lock()
	d.latencyTotal = 0
	d.latencyCount = 0
	d.latencyMu.Unlock()

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
	fmt.Fprintf(&sb, "  messages    : %d total, %s\n", s.TotalMessages, formatBytes(s.TotalMessageBytes))
	fmt.Fprintf(&sb, "  rooms       : %d joins, %d leaves\n", s.TotalRoomJoins, s.TotalRoomLeaves)

	if s.AvgLatency > 0 {
		fmt.Fprintf(&sb, "  avg latency : %s\n", s.AvgLatency)
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
