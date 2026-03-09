import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function MetricsDocs() {
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
        'Easy to integrate with Prometheus, StatsD, or custom backends',
      ]}
    >
      {/* ── Metrics Interface ── */}
      <h3 id="metrics-interface" className="text-lg font-semibold text-text-heading mt-8 mb-2">Metrics Interface</h3>
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

      {/* ── Debug Metrics ── */}
      <h3 id="metrics-debug" className="text-lg font-semibold text-text-heading mt-8 mb-2">Debug Metrics</h3>
      <p className="text-text-muted mb-3">
        Built-in debug implementation for development and testing:
      </p>
      <CodeBlock code={`// Create debug metrics collector
metrics := wshub.NewDebugMetrics()

hub := wshub.NewHub(
    wshub.WithMetrics(metrics),
)

// Get a point-in-time stats snapshot
stats := metrics.Stats()
fmt.Printf("Connections: %d\\n", stats.Connections)
fmt.Printf("Messages: %d\\n", stats.Messages)

// Pretty-print summary
fmt.Println(metrics)`} />
    </ModuleSection>
  );
}
