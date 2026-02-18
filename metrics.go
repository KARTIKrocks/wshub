package wshub

import "time"

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
