import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function MiddlewareDocs() {
  return (
    <ModuleSection
      id="middleware"
      title="Middleware"
      description="Chain message handlers with custom logic using the middleware pattern. Middleware wraps the message handler to add cross-cutting concerns."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Composable middleware chain',
        'Built-in logging, recovery, and metrics middleware',
        'Easy to write custom middleware',
      ]}
    >
      {/* ── Middleware Chain ── */}
      <h3 id="middleware-chain" className="text-lg font-semibold text-text-heading mt-8 mb-2">Middleware Chain</h3>
      <CodeBlock code={`type Middleware func(HandlerFunc) HandlerFunc
type HandlerFunc func(*Client, *Message) error

// Build a middleware chain
chain := wshub.NewMiddlewareChain(finalHandler).
    Use(wshub.RecoveryMiddleware(logger)).
    Use(wshub.LoggingMiddleware(logger)).
    Use(wshub.MetricsMiddleware(metrics)).
    Build()

// Use with the hub
hub := wshub.NewHub(
    wshub.WithMessageHandler(chain),
)`} />

      {/* ── Built-in Middleware ── */}
      <h3 id="middleware-builtin" className="text-lg font-semibold text-text-heading mt-8 mb-2">Built-in Middleware</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Middleware</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">LoggingMiddleware(logger)</td><td className="py-2 text-text-muted">Log message events</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">RecoveryMiddleware(logger)</td><td className="py-2 text-text-muted">Recover from panics in message handlers</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">MetricsMiddleware(metrics)</td><td className="py-2 text-text-muted">Record message processing metrics</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Use built-in middleware
chain := wshub.NewMiddlewareChain(handler).
    Use(wshub.RecoveryMiddleware(logger)).   // catch panics
    Use(wshub.LoggingMiddleware(logger)).    // log messages
    Use(wshub.MetricsMiddleware(metrics)).   // record metrics
    Build()`} />

      {/* ── Custom Middleware ── */}
      <h3 id="middleware-custom" className="text-lg font-semibold text-text-heading mt-8 mb-2">Custom Middleware</h3>
      <p className="text-text-muted mb-3">
        Write custom middleware by implementing the Middleware signature:
      </p>
      <CodeBlock code={`// Custom middleware that filters messages
func ProfanityFilter(next wshub.HandlerFunc) wshub.HandlerFunc {
    return func(client *wshub.Client, msg *wshub.Message) error {
        if containsProfanity(msg.Text()) {
            client.SendText("Message blocked: inappropriate content")
            return nil // swallow the message
        }
        return next(client, msg)
    }
}

// Custom middleware that adds timing
func TimingMiddleware(next wshub.HandlerFunc) wshub.HandlerFunc {
    return func(client *wshub.Client, msg *wshub.Message) error {
        start := time.Now()
        err := next(client, msg)
        log.Printf("Message processed in %v", time.Since(start))
        return err
    }
}

// Use in chain
chain := wshub.NewMiddlewareChain(handler).
    Use(ProfanityFilter).
    Use(TimingMiddleware).
    Build()`} />
    </ModuleSection>
  );
}
