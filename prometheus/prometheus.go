// Package prometheus provides a Prometheus [wshub.MetricsCollector] for wshub.
//
// Usage:
//
//	reg := prometheus.NewRegistry()
//	collector := wshubprom.New(wshubprom.WithRegistry(reg))
//	hub := wshub.NewHub(wshub.WithMetrics(collector))
//	go hub.Run()
//	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
package prometheus

import (
	"time"

	"github.com/KARTIKrocks/wshub"
	"github.com/prometheus/client_golang/prometheus"
)

// compile-time interface check.
var _ wshub.MetricsCollector = (*Collector)(nil)

// Option configures a [Collector].
type Option func(*Collector)

// WithRegistry sets a custom Prometheus registerer. Default: [prometheus.DefaultRegisterer].
func WithRegistry(reg prometheus.Registerer) Option {
	return func(c *Collector) {
		c.registerer = reg
	}
}

// WithNamespace sets the metric namespace (prefix). Default: "wshub".
func WithNamespace(ns string) Option {
	return func(c *Collector) {
		c.namespace = ns
	}
}

// WithLatencyBuckets sets custom histogram buckets for message latency.
// Default: [prometheus.DefBuckets].
func WithLatencyBuckets(buckets []float64) Option {
	return func(c *Collector) {
		c.latencyBuckets = buckets
	}
}

// WithBroadcastBuckets sets custom histogram buckets for broadcast duration.
// Default: {.0001, .0005, .001, .005, .01, .05, .1, .5, 1}.
func WithBroadcastBuckets(buckets []float64) Option {
	return func(c *Collector) {
		c.broadcastBuckets = buckets
	}
}

// Collector implements [wshub.MetricsCollector] using Prometheus metrics.
// It is safe for concurrent use.
type Collector struct {
	registerer       prometheus.Registerer
	namespace        string
	latencyBuckets   []float64
	broadcastBuckets []float64

	connectionsActive prometheus.Gauge
	connectionsTotal  prometheus.Counter
	messagesReceived  prometheus.Counter
	messagesSent      prometheus.Counter
	messagesDropped   prometheus.Counter
	messageBytes      prometheus.Counter
	latency           prometheus.Histogram
	broadcastDuration prometheus.Histogram
	roomsActive       prometheus.Gauge
	roomJoins         prometheus.Counter
	roomLeaves        prometheus.Counter
	errors            *prometheus.CounterVec
}

// New creates a new Prometheus [Collector] and registers all metrics.
// It panics if any metric fails to register (e.g. duplicate names).
func New(opts ...Option) *Collector {
	c := &Collector{
		registerer:       prometheus.DefaultRegisterer,
		namespace:        "wshub",
		latencyBuckets:   prometheus.DefBuckets,
		broadcastBuckets: []float64{.0001, .0005, .001, .005, .01, .05, .1, .5, 1},
	}
	for _, opt := range opts {
		opt(c)
	}

	c.connectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: c.namespace,
		Name:      "connections_active",
		Help:      "Number of active WebSocket connections.",
	})
	c.connectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "connections_total",
		Help:      "Total number of WebSocket connections opened.",
	})
	c.messagesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "messages_received_total",
		Help:      "Total number of messages received from clients.",
	})
	c.messagesSent = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "messages_sent_total",
		Help:      "Total number of messages sent to clients.",
	})
	c.messagesDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "messages_dropped_total",
		Help:      "Total number of messages dropped due to full send buffers.",
	})
	c.messageBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "message_received_bytes_total",
		Help:      "Total bytes received from clients.",
	})
	c.latency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: c.namespace,
		Name:      "message_latency_seconds",
		Help:      "Message handler processing latency in seconds.",
		Buckets:   c.latencyBuckets,
	})
	c.broadcastDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: c.namespace,
		Name:      "broadcast_duration_seconds",
		Help:      "Duration of local broadcast fanout in seconds.",
		Buckets:   c.broadcastBuckets,
	})
	c.roomsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: c.namespace,
		Name:      "rooms_active",
		Help:      "Number of active rooms.",
	})
	c.roomJoins = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "room_joins_total",
		Help:      "Total number of room join operations.",
	})
	c.roomLeaves = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "room_leaves_total",
		Help:      "Total number of room leave operations.",
	})
	c.errors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: c.namespace,
		Name:      "errors_total",
		Help:      "Total number of errors by type.",
	}, []string{"type"})

	c.registerer.MustRegister(
		c.connectionsActive, c.connectionsTotal,
		c.messagesReceived, c.messagesSent, c.messagesDropped,
		c.messageBytes,
		c.latency, c.broadcastDuration,
		c.roomsActive, c.roomJoins, c.roomLeaves,
		c.errors,
	)

	return c
}

// IncrementConnections increments both active and total connection counters.
func (c *Collector) IncrementConnections() {
	c.connectionsActive.Inc()
	c.connectionsTotal.Inc()
}

// DecrementConnections decrements the active connections gauge.
func (c *Collector) DecrementConnections() {
	c.connectionsActive.Dec()
}

// IncrementMessagesReceived increments the counter for messages received from clients.
func (c *Collector) IncrementMessagesReceived() {
	c.messagesReceived.Inc()
}

// IncrementMessagesSent increments the counter for messages sent to clients by the given count.
func (c *Collector) IncrementMessagesSent(count int) {
	c.messagesSent.Add(float64(count))
}

// IncrementMessagesDropped increments the counter for messages dropped due to full send buffers.
func (c *Collector) IncrementMessagesDropped() {
	c.messagesDropped.Inc()
}

// RecordMessageSize adds the given message size to the total bytes received counter.
func (c *Collector) RecordMessageSize(size int) {
	c.messageBytes.Add(float64(size))
}

// RecordLatency observes a message handler processing latency in the histogram.
func (c *Collector) RecordLatency(duration time.Duration) {
	c.latency.Observe(duration.Seconds())
}

// RecordBroadcastDuration observes a broadcast fanout duration in the histogram.
func (c *Collector) RecordBroadcastDuration(duration time.Duration) {
	c.broadcastDuration.Observe(duration.Seconds())
}

// IncrementRoomJoins increments the counter for room join operations.
func (c *Collector) IncrementRoomJoins() {
	c.roomJoins.Inc()
}

// IncrementRoomLeaves increments the counter for room leave operations.
func (c *Collector) IncrementRoomLeaves() {
	c.roomLeaves.Inc()
}

// IncrementRooms increments the active rooms gauge.
func (c *Collector) IncrementRooms() {
	c.roomsActive.Inc()
}

// DecrementRooms decrements the active rooms gauge.
func (c *Collector) DecrementRooms() {
	c.roomsActive.Dec()
}

// IncrementErrors increments the error counter for the given error type.
func (c *Collector) IncrementErrors(errorType string) {
	c.errors.WithLabelValues(errorType).Inc()
}
