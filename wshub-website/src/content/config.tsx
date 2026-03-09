import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function ConfigDocs() {
  return (
    <ModuleSection
      id="config"
      title="Configuration"
      description="Extensive WebSocket configuration using the builder pattern for buffer sizes, timeouts, compression, and origin checking."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Sensible defaults out of the box',
        'Builder pattern for fluent configuration',
        'Configurable buffer sizes, timeouts, and message limits',
        'Per-message compression support',
        'Pluggable origin validation',
      ]}
    >
      {/* ── Default Config ── */}
      <h3 id="config-defaults" className="text-lg font-semibold text-text-heading mt-8 mb-2">Default Config</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Option</th>
              <th className="py-2 text-text-heading font-semibold">Default</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ReadBufferSize</td><td className="py-2 text-text-muted">1024</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WriteBufferSize</td><td className="py-2 text-text-muted">1024</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WriteWait</td><td className="py-2 text-text-muted">10s</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">PongWait</td><td className="py-2 text-text-muted">60s</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">PingPeriod</td><td className="py-2 text-text-muted">54s (90% of PongWait)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">MaxMessageSize</td><td className="py-2 text-text-muted">512 KB</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendChannelSize</td><td className="py-2 text-text-muted">256</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">EnableCompression</td><td className="py-2 text-text-muted">false</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Use default config
config := wshub.DefaultConfig()

// Use with hub
hub := wshub.NewHub(
    wshub.WithConfig(config),
)`} />

      {/* ── Builder Methods ── */}
      <h3 id="config-builder" className="text-lg font-semibold text-text-heading mt-8 mb-2">Builder Methods</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithBufferSizes(read, write)</td><td className="py-2 text-text-muted">Set read and write buffer sizes</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithMaxMessageSize(size)</td><td className="py-2 text-text-muted">Set maximum message size in bytes</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithCompression(enabled)</td><td className="py-2 text-text-muted">Enable per-message compression</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithCheckOrigin(fn)</td><td className="py-2 text-text-muted">Set origin validation function</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`config := wshub.DefaultConfig().
    WithBufferSizes(4096, 4096).
    WithMaxMessageSize(1024 * 1024). // 1 MB
    WithCompression(true).
    WithCheckOrigin(wshub.AllowOrigins("https://example.com"))

hub := wshub.NewHub(
    wshub.WithConfig(config),
)`} />

      {/* ── Origin Checking ── */}
      <h3 id="config-origins" className="text-lg font-semibold text-text-heading mt-8 mb-2">Origin Checking</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Function</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AllowAllOrigins()</td><td className="py-2 text-text-muted">Allow connections from any origin</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AllowSameOrigin()</td><td className="py-2 text-text-muted">Only allow same-origin connections</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AllowOrigins(origins...)</td><td className="py-2 text-text-muted">Allow specific origins</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Allow all origins (development)
config.WithCheckOrigin(wshub.AllowAllOrigins())

// Same-origin only
config.WithCheckOrigin(wshub.AllowSameOrigin())

// Specific origins
config.WithCheckOrigin(wshub.AllowOrigins(
    "https://example.com",
    "https://app.example.com",
))`} />
    </ModuleSection>
  );
}
