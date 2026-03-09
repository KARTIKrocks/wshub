import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function HubDocs() {
  return (
    <ModuleSection
      id="hub"
      title="Hub"
      description="The central connection manager that handles all WebSocket clients, rooms, and message routing."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Manages all connected WebSocket clients',
        'Supports broadcasting to all, specific clients, or rooms',
        'O(1) client and user lookups via hash maps',
        'Snapshot-based lock-free broadcasting',
        'Optional parallel broadcasting for 1000+ clients',
        'Graceful shutdown with context support',
      ]}
    >
      {/* ── Creating a Hub ── */}
      <h3 id="hub-creating" className="text-lg font-semibold text-text-heading mt-8 mb-2">Creating a Hub</h3>
      <p className="text-text-muted mb-3">
        Create a hub with functional options and start the run loop:
      </p>
      <CodeBlock code={`hub := wshub.NewHub(
    wshub.WithConfig(config),
    wshub.WithLogger(logger),
    wshub.WithMetrics(metrics),
    wshub.WithLimits(limits),
    wshub.WithHooks(hooks),
    wshub.WithMessageHandler(handler),
    wshub.WithParallelBroadcast(100), // batch size for parallel broadcast
)

// Start the hub run loop (required)
go hub.Run()

// Register as HTTP handler
http.HandleFunc("/ws", hub.HandleHTTP())`} />

      {/* ── Hub Options ── */}
      <h3 id="hub-options" className="text-lg font-semibold text-text-heading mt-8 mb-2">Hub Options</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Option</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithConfig(cfg)</td><td className="py-2 text-text-muted">Set WebSocket configuration (buffer sizes, timeouts, compression)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithLogger(l)</td><td className="py-2 text-text-muted">Set a custom logger implementation</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithMetrics(m)</td><td className="py-2 text-text-muted">Set a metrics collector</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithLimits(l)</td><td className="py-2 text-text-muted">Set connection and rate limits</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithHooks(h)</td><td className="py-2 text-text-muted">Set lifecycle hooks</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithMessageHandler(fn)</td><td className="py-2 text-text-muted">Set the message handler function</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithParallelBroadcast(n)</td><td className="py-2 text-text-muted">Enable parallel broadcasting with batch size n</td></tr>
          </tbody>
        </table>
      </div>

      {/* ── Broadcasting ── */}
      <h3 id="hub-broadcasting" className="text-lg font-semibold text-text-heading mt-8 mb-2">Broadcasting</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Broadcast(data)</td><td className="py-2 text-text-muted">Send bytes to all connected clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastText(text)</td><td className="py-2 text-text-muted">Send a text string to all clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastBinary(data)</td><td className="py-2 text-text-muted">Send binary data to all clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastJSON(v)</td><td className="py-2 text-text-muted">JSON-encode and send to all clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastWithContext(ctx, data)</td><td className="py-2 text-text-muted">Broadcast with context support</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastExcept(data, except...)</td><td className="py-2 text-text-muted">Send to all except specified clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendToClient(clientID, data)</td><td className="py-2 text-text-muted">Send to a specific client by ID</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendToUser(userID, data)</td><td className="py-2 text-text-muted">Send to all connections of a user</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Broadcast to all connected clients
hub.Broadcast([]byte("hello everyone"))
hub.BroadcastText("hello everyone")
hub.BroadcastJSON(map[string]string{"type": "notification", "msg": "hello"})

// Send to specific client or user
hub.SendToClient(clientID, []byte("private message"))
hub.SendToUser(userID, []byte("sent to all devices"))

// Broadcast to all except certain clients
hub.BroadcastExcept([]byte("hello others"), excludedClient1, excludedClient2)`} />

      {/* ── Client Lookup ── */}
      <h3 id="hub-client-lookup" className="text-lg font-semibold text-text-heading mt-8 mb-2">Client Lookup</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Clients()</td><td className="py-2 text-text-muted">Get all connected clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ClientCount()</td><td className="py-2 text-text-muted">Get count of connected clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetClient(id)</td><td className="py-2 text-text-muted">O(1) client lookup by ID</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetClientByUserID(userID)</td><td className="py-2 text-text-muted">Get first client for a user</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetClientsByUserID(userID)</td><td className="py-2 text-text-muted">Get all connections for a user</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Look up clients
client, ok := hub.GetClient(clientID)
if ok {
    client.SendText("found you!")
}

// Multi-device: get all connections for a user
clients := hub.GetClientsByUserID("user-123")
for _, c := range clients {
    c.SendJSON(map[string]string{"type": "sync"})
}

// Count and list
count := hub.ClientCount()
allClients := hub.Clients()`} />

      {/* ── Graceful Shutdown ── */}
      <h3 id="hub-shutdown" className="text-lg font-semibold text-text-heading mt-8 mb-2">Graceful Shutdown</h3>
      <p className="text-text-muted mb-3">
        The hub supports context-aware graceful shutdown that closes all client connections:
      </p>
      <CodeBlock code={`ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Gracefully shutdown the hub — closes all connections
if err := hub.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}`} />
    </ModuleSection>
  );
}
