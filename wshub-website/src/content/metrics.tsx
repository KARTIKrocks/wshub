import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';
import { useVersion } from '../hooks/useVersion';

export default function MetricsDocs() {
  const { minVersion } = useVersion();
  const v150 = minVersion('v1.5.0');

  return (
    <ModuleSection
      id="metrics"
      title="Metrics"
      description="Pluggable metrics collection interface for observability. Implement the MetricsCollector interface or use the built-in debug implementation."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Pluggable MetricsCollector interface',
        'Built-in DebugMetrics implementation for development',
        'Track connections, messages, latency, errors, and room events',
        ...(v150 ? ['Official Prometheus subpackage with drop-in MetricsCollector'] : ['Easy to integrate with Prometheus, StatsD, or custom backends']),
      ]}
    >
      {/* ── Metrics Interface ── */}
      <h3 id="metrics-interface" className="text-lg font-semibold text-text-heading mt-8 mb-2">Metrics Interface</h3>
      {v150 ? (
        <CodeBlock code={`type MetricsCollector interface {
    IncrementConnections()
    DecrementConnections()
    IncrementMessagesReceived()        // renamed from IncrementMessages in v1.5.0
    IncrementMessagesSent(count int)   // new in v1.5.0
    IncrementMessagesDropped()         // new in v1.5.0
    RecordMessageSize(size int)
    RecordLatency(duration time.Duration)
    RecordBroadcastDuration(duration time.Duration) // new in v1.5.0
    IncrementErrors(errorType string)
    IncrementRoomJoins()
    IncrementRoomLeaves()
    IncrementRooms()                   // new in v1.5.0
    DecrementRooms()                   // new in v1.5.0
}

// Use with hub
hub := wshub.NewHub(
    wshub.WithMetrics(myCollector),
)`} />
      ) : (
        <CodeBlock code={`type MetricsCollector interface {
    IncrementConnections()
    DecrementConnections()
    IncrementMessages()
    RecordMessageSize(size int)
    RecordLatency(duration time.Duration)
    IncrementErrors(errorType string)
    IncrementRoomJoins()
    IncrementRoomLeaves()
}

// Use with hub
hub := wshub.NewHub(
    wshub.WithMetrics(myPrometheusCollector),
)`} />
      )}

      {/* ── Debug Metrics ── */}
      <h3 id="metrics-debug" className="text-lg font-semibold text-text-heading mt-8 mb-2">Debug Metrics</h3>
      <p className="text-text-muted mb-3">
        Built-in debug implementation for development and testing:
      </p>
      {v150 ? (
        <CodeBlock code={`// Create debug metrics collector
metrics := wshub.NewDebugMetrics()

hub := wshub.NewHub(
    wshub.WithMetrics(metrics),
)

// Get a point-in-time stats snapshot
stats := metrics.Stats()
fmt.Printf("Connections: %d\\n", stats.Connections)
fmt.Printf("Received: %d\\n", stats.TotalMessagesRecv) // renamed from TotalMessages
fmt.Printf("Sent: %d\\n", stats.TotalMessagesSent)
fmt.Printf("Dropped: %d\\n", stats.TotalDropped)
fmt.Printf("Active rooms: %d\\n", stats.ActiveRooms)
fmt.Printf("Avg broadcast: %v\\n", stats.AvgBroadcast)

// Pretty-print summary
fmt.Println(metrics)`} />
      ) : (
        <CodeBlock code={`// Create debug metrics collector
metrics := wshub.NewDebugMetrics()

hub := wshub.NewHub(
    wshub.WithMetrics(metrics),
)

// Get a point-in-time stats snapshot
stats := metrics.Stats()
fmt.Printf("Connections: %d\\n", stats.Connections)
fmt.Printf("Messages: %d\\n", stats.TotalMessages)

// Pretty-print summary
fmt.Println(metrics)`} />
      )}

      {/* ── Prometheus Subpackage (v1.5.0+) ── */}
      {v150 && <>
        <h3 id="metrics-prometheus" className="text-lg font-semibold text-text-heading mt-8 mb-2">Prometheus Subpackage</h3>
        <p className="text-text-muted mb-3">
          Official drop-in <code className="text-accent">MetricsCollector</code> backed by <code className="text-accent">prometheus/client_golang</code>.
          Import as a separate module:
        </p>
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left">
                <th className="py-2 pr-4 text-text-heading font-semibold">Option</th>
                <th className="py-2 text-text-heading font-semibold">Description</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithRegistry(reg)</td><td className="py-2 text-text-muted">Use a custom Prometheus registry (default: prometheus.DefaultRegisterer)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithNamespace(ns)</td><td className="py-2 text-text-muted">Set metric name prefix (default: "wshub")</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithLatencyBuckets(buckets)</td><td className="py-2 text-text-muted">Custom histogram buckets for message_latency_seconds</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithBroadcastBuckets(buckets)</td><td className="py-2 text-text-muted">Custom histogram buckets for broadcast_duration_seconds</td></tr>
            </tbody>
          </table>
        </div>
        <p className="text-text-muted mb-3">Exposed metrics:</p>
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left">
                <th className="py-2 pr-4 text-text-heading font-semibold">Metric</th>
                <th className="py-2 text-text-heading font-semibold">Description</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">connections_active</td><td className="py-2 text-text-muted">Gauge of currently connected clients</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">connections_total</td><td className="py-2 text-text-muted">Counter of all connections ever accepted</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">messages_received_total</td><td className="py-2 text-text-muted">Counter of inbound messages</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">messages_sent_total</td><td className="py-2 text-text-muted">Counter of outbound messages</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">messages_dropped_total</td><td className="py-2 text-text-muted">Counter of messages dropped due to full send buffers</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">message_received_bytes_total</td><td className="py-2 text-text-muted">Counter of inbound bytes</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">message_latency_seconds</td><td className="py-2 text-text-muted">Histogram of message handler latency</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">broadcast_duration_seconds</td><td className="py-2 text-text-muted">Histogram of local fanout duration</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">rooms_active</td><td className="py-2 text-text-muted">Gauge of currently active rooms</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">room_joins_total</td><td className="py-2 text-text-muted">Counter of room join events</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">room_leaves_total</td><td className="py-2 text-text-muted">Counter of room leave events</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">errors_total{'{type}'}</td><td className="py-2 text-text-muted">Counter of errors labelled by type</td></tr>
            </tbody>
          </table>
        </div>
        <CodeBlock code={`import whprom "github.com/KARTIKrocks/wshub/prometheus"

// Default setup — registers on prometheus.DefaultRegisterer
collector := whprom.NewCollector()

// Custom registry and namespace
collector := whprom.NewCollector(
    whprom.WithRegistry(myRegistry),
    whprom.WithNamespace("myapp"),
)

hub := wshub.NewHub(
    wshub.WithMetrics(collector),
)`} />
      </>}
    </ModuleSection>
  );
}
