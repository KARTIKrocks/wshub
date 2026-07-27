import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';
import { useVersion } from '../hooks/useVersion';

export default function ConfigDocs() {
  const { minVersion } = useVersion();
  const v130 = minVersion('v1.3.0');
  const v170 = minVersion('v1.7.0');
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
        ...(v130 ? ['Opt-in write coalescing for high-throughput text broadcasts'] : []),
        v170 ? 'Same-origin checking on by default' : 'Pluggable origin validation',
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
            {v130 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">CoalesceWrites</td><td className="py-2 text-text-muted">false</td></tr>}
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">CheckOrigin</td><td className="py-2 text-text-muted">{v170 ? 'AllowSameOrigin' : 'AllowAllOrigins'}</td></tr>
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
            {v130 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithCoalesceWrites(enabled)</td><td className="py-2 text-text-muted">Batch queued text messages into a single WebSocket frame (separated by \n), reducing syscalls under high throughput</td></tr>}
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithCheckOrigin(fn)</td><td className="py-2 text-text-muted">Set origin validation function</td></tr>
          </tbody>
        </table>
      </div>
      {v130 ? (
        <CodeBlock code={`config := wshub.DefaultConfig().
    WithBufferSizes(4096, 4096).
    WithMaxMessageSize(1024 * 1024). // 1 MB
    WithCompression(true).
    WithCoalesceWrites(true). // batch text messages into single frames
    WithCheckOrigin(wshub.AllowOrigins("https://example.com"))

hub := wshub.NewHub(
    wshub.WithConfig(config),
)`} />
      ) : (
        <CodeBlock code={`config := wshub.DefaultConfig().
    WithBufferSizes(4096, 4096).
    WithMaxMessageSize(1024 * 1024). // 1 MB
    WithCompression(true).
    WithCheckOrigin(wshub.AllowOrigins("https://example.com"))

hub := wshub.NewHub(
    wshub.WithConfig(config),
)`} />
      )}

      {/* ── Origin Checking ── */}
      <h3 id="config-origins" className="text-lg font-semibold text-text-heading mt-8 mb-2">Origin Checking</h3>
      {v170 ? (
        <p className="text-text-muted mb-3">
          Since v1.7.0 the default is <span className="font-mono text-accent">AllowSameOrigin</span>.
          Earlier versions defaulted to <span className="font-mono text-accent">AllowAllOrigins</span>,
          which let any page on any site open an authenticated connection using the
          visitor&apos;s cookies (cross-site WebSocket hijacking). If your front-end is served
          from a different origin than the WebSocket endpoint, allowlist it with{' '}
          <span className="font-mono text-accent">AllowOrigins</span> — otherwise those
          upgrades are rejected with <span className="font-mono text-accent">403</span> and
          an <span className="font-mono text-accent">origin_rejected</span> metric.
        </p>
      ) : (
        <p className="text-text-muted mb-3">
          This version defaults to <span className="font-mono text-accent">AllowAllOrigins</span>,
          which accepts an upgrade from any origin. Set an explicit checker in production —
          v1.7.0 changed the default to <span className="font-mono text-accent">AllowSameOrigin</span>.
        </p>
      )}
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Function</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AllowAllOrigins</td><td className="py-2 text-text-muted">Allow connections from any origin — development only</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AllowSameOrigin</td><td className="py-2 text-text-muted">Only allow same-origin connections{v170 ? ' (the default)' : ''}</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AllowOrigins(origins...)</td><td className="py-2 text-text-muted">Allow specific origins, compared as full origin strings</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Same-origin only${v170 ? ' (the default — no call needed)' : ''}
config.WithCheckOrigin(wshub.AllowSameOrigin)

// Specific origins — use this when your front-end is served from a
// different host than the WebSocket endpoint
config.WithCheckOrigin(wshub.AllowOrigins(
    "https://example.com",
    "https://app.example.com",
))

// Custom checker
config.WithCheckOrigin(func(r *http.Request) bool {
    return strings.HasSuffix(r.Header.Get("Origin"), ".example.com")
})

// Disable the check entirely — development only
config.WithCheckOrigin(wshub.AllowAllOrigins)`} />
      <p className="text-text-muted mt-3">
        Requests with no <span className="font-mono text-accent">Origin</span> header are allowed
        by both <span className="font-mono text-accent">AllowSameOrigin</span> and{' '}
        <span className="font-mono text-accent">AllowOrigins</span>, since non-browser clients
        (mobile apps, CLI tools, server-to-server) typically omit it. Browsers always send it,
        so the cross-site hijacking path stays closed.
      </p>
      {v170 && (
        <p className="text-text-muted mt-3">
          <span className="font-mono text-accent">AllowSameOrigin</span> compares host and port,
          not scheme, so <span className="font-mono text-accent">http://example.com</span> is
          accepted by a server reachable at <span className="font-mono text-accent">example.com</span>{' '}
          over https. A server behind a TLS-terminating proxy cannot see its own scheme, so
          comparing it would reject the legitimate origins of every proxied deployment. Use{' '}
          <span className="font-mono text-accent">AllowOrigins</span> if you need scheme-exact
          matching.
        </p>
      )}
    </ModuleSection>
  );
}
