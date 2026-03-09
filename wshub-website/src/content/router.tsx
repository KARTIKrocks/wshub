import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function RouterDocs() {
  return (
    <ModuleSection
      id="router"
      title="Router"
      description="Dispatch incoming messages to per-event handlers based on an event name extracted from each message. The router is format-agnostic — JSON, msgpack, binary, or anything else."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Event-based message dispatching',
        'Format-agnostic extractor function',
        'Chainable handler registration',
        'Fallback handler for unmatched events',
      ]}
    >
      {/* ── Creating a Router ── */}
      <h3 id="router-creating" className="text-lg font-semibold text-text-heading mt-8 mb-2">Creating a Router</h3>
      <p className="text-text-muted mb-3">
        Create a router with an extractor function that determines the event name from each message:
      </p>
      <CodeBlock code={`router := wshub.NewRouter(func(msg *wshub.Message) string {
    var env struct{ Type string \`json:"type"\` }
    json.Unmarshal(msg.Data, &env)
    return env.Type
})

// Register handlers and use with a hub
hub := wshub.NewHub(wshub.WithMessageHandler(router.Handle))`} />

      {/* ── Registering Handlers ── */}
      <h3 id="router-handlers" className="text-lg font-semibold text-text-heading mt-8 mb-2">Registering Handlers</h3>
      <p className="text-text-muted mb-3">
        Use <code className="text-accent">On()</code> to register handlers for specific events. Calls can be chained:
      </p>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">On(event, handler)</td><td className="py-2 text-text-muted">Register a handler for the given event name</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">OnNotFound(handler)</td><td className="py-2 text-text-muted">Set a fallback handler for unmatched events (defaults to returning ErrInvalidMessage)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Handle(client, msg)</td><td className="py-2 text-text-muted">Dispatch a message to the appropriate handler — pass to WithMessageHandler</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`router.
    On("chat",  handleChat).
    On("join",  handleJoin).
    On("leave", handleLeave).
    OnNotFound(func(client *wshub.Client, msg *wshub.Message) error {
        return client.SendText("unknown event")
    })`} />

      {/* ── Full Example ── */}
      <h3 id="router-example" className="text-lg font-semibold text-text-heading mt-8 mb-2">Full Example</h3>
      <p className="text-text-muted mb-3">
        A complete example using the router with rooms for a chat application:
      </p>
      <CodeBlock code={`func main() {
    router := wshub.NewRouter(func(msg *wshub.Message) string {
        var env struct{ Type string \`json:"type"\` }
        json.Unmarshal(msg.Data, &env)
        return env.Type
    })

    router.
        On("chat", func(c *wshub.Client, msg *wshub.Message) error {
            return c.Hub().BroadcastText(msg.Text(), nil)
        }).
        On("join", func(c *wshub.Client, msg *wshub.Message) error {
            var req struct{ Room string \`json:"room"\` }
            json.Unmarshal(msg.Data, &req)
            return c.Hub().JoinRoom(c, req.Room)
        }).
        On("leave", func(c *wshub.Client, msg *wshub.Message) error {
            var req struct{ Room string \`json:"room"\` }
            json.Unmarshal(msg.Data, &req)
            return c.Hub().LeaveRoom(c, req.Room)
        })

    hub := wshub.NewHub(
        wshub.WithMessageHandler(router.Handle),
    )

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    go hub.Run(ctx)
}`} />
    </ModuleSection>
  );
}
