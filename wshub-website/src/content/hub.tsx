import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';
import { useVersion } from '../hooks/useVersion';

export default function HubDocs() {
  const { minVersion } = useVersion();
  const v110 = minVersion('v1.1.0');
  const v112 = minVersion('v1.1.2');
  const v113 = minVersion('v1.1.3');
  const v120 = minVersion('v1.2.0');

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
        ...(v110 ? [
          'Multi-node support via pluggable adapters',
          'Configurable backpressure with drop policies',
        ] : []),
        'Graceful shutdown with context support',
      ]}
    >
      {/* ── Creating a Hub ── */}
      <h3 id="hub-creating" className="text-lg font-semibold text-text-heading mt-8 mb-2">Creating a Hub</h3>
      <p className="text-text-muted mb-3">
        Create a hub with functional options and start the run loop:
      </p>
      {v110 ? (
        <CodeBlock code={`hub := wshub.NewHub(
    wshub.WithConfig(config),
    wshub.WithLogger(logger),
    wshub.WithMetrics(metrics),
    wshub.WithLimits(limits),
    wshub.WithHooks(hooks),
    wshub.WithMessageHandler(handler),
    wshub.WithParallelBroadcast(100), // batch size for parallel broadcast
    wshub.WithAdapter(adapter),       // multi-node support
    wshub.WithPresence(5*time.Second),// cluster-wide counts
    wshub.WithDropPolicy(wshub.DropOldest), // backpressure control
)

// Start the hub run loop (required)
go hub.Run()

// Register as HTTP handler
http.HandleFunc("/ws", hub.HandleHTTP())

// UpgradeConnection with options (e.g., set user ID atomically)
client, err := hub.UpgradeConnection(w, r, wshub.WithUserID("user-123"))`} />
      ) : (
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
      )}

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
            {v112 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithParallelBroadcastWorkers(n)</td><td className="py-2 text-text-muted">Set number of persistent worker goroutines for parallel broadcasting (default: runtime.NumCPU()). No effect unless WithParallelBroadcast is also set</td></tr>}
            {v110 && <>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithAdapter(adapter)</td><td className="py-2 text-text-muted">Set multi-node adapter for cross-node message delivery</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithNodeID(id)</td><td className="py-2 text-text-muted">Set a stable node identifier for debugging (default: random UUID)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithPresence(interval)</td><td className="py-2 text-text-muted">Enable periodic presence gossip for global counts</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithHookTimeout(d)</td><td className="py-2 text-text-muted">Max wait for synchronous hooks like BeforeDisconnect (default: 5s)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithDropPolicy(policy)</td><td className="py-2 text-text-muted">Set backpressure behavior when send buffer is full (DropNewest or DropOldest)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithoutHandlerLatency()</td><td className="py-2 text-text-muted">Disable built-in latency recording (use with MetricsMiddleware to avoid double-counting)</td></tr>
            </>}
            {v120 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithDrainTimeout(d)</td><td className="py-2 text-text-muted">Max idle time before closing a connection during drain (default: 30s). Set to 0 to disable the idle reaper</td></tr>}
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
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Broadcast(data)</td><td className="py-2 text-text-muted">Send text bytes to all connected clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastText(text)</td><td className="py-2 text-text-muted">Send a text string to all clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastBinary(data)</td><td className="py-2 text-text-muted">Send binary data to all clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastJSON(v)</td><td className="py-2 text-text-muted">JSON-encode and send to all clients</td></tr>
            {v113 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastRawJSON(data)</td><td className="py-2 text-text-muted">Broadcast pre-serialized JSON bytes to all clients (0 allocs, skips marshaling)</td></tr>}
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastWithContext(ctx, data)</td><td className="py-2 text-text-muted">Broadcast with context support</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastExcept(data, except...)</td><td className="py-2 text-text-muted">Send text to all except specified clients</td></tr>
            {v110 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastBinaryExcept(data, except...)</td><td className="py-2 text-text-muted">Send binary to all except specified clients</td></tr>}
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendToClient(clientID, data)</td><td className="py-2 text-text-muted">Send text to a specific client by ID</td></tr>
            {v110 && <>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendBinaryToClient(clientID, data)</td><td className="py-2 text-text-muted">Send binary to a specific client by ID</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendToClientWithContext(ctx, clientID, data)</td><td className="py-2 text-text-muted">Send to client with context (blocks until enqueued)</td></tr>
            </>}
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendToUser(userID, data)</td><td className="py-2 text-text-muted">Send text to all connections of a user</td></tr>
            {v110 && <>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendBinaryToUser(userID, data)</td><td className="py-2 text-text-muted">Send binary to all connections of a user</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendToUserWithContext(ctx, userID, data)</td><td className="py-2 text-text-muted">Send to user with context (blocks until enqueued)</td></tr>
            </>}
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
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ClientCount()</td><td className="py-2 text-text-muted">Get count of connected clients{v110 ? ' (atomic, no lock)' : ''}</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetClient(id)</td><td className="py-2 text-text-muted">O(1) client lookup by ID</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetClientByUserID(userID)</td><td className="py-2 text-text-muted">Get first client for a user</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetClientsByUserID(userID)</td><td className="py-2 text-text-muted">Get all connections for a user</td></tr>
            {v110 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">NodeID()</td><td className="py-2 text-text-muted">Get this hub's unique node identifier</td></tr>}
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

      {/* ── Upgrade Options (v1.1.0+) ── */}
      {v110 && <>
        <h3 id="hub-upgrade" className="text-lg font-semibold text-text-heading mt-8 mb-2">Upgrade Options</h3>
        <p className="text-text-muted mb-3">
          Pass per-connection options to <code className="text-accent">UpgradeConnection</code> to configure the client before registration:
        </p>
        <CodeBlock code={`// Set user ID atomically during upgrade — before registration
// This prevents the window where a client exists without a user ID,
// which could bypass MaxConnectionsPerUser limits
client, err := hub.UpgradeConnection(w, r, wshub.WithUserID(userID))
if err != nil {
    log.Printf("Upgrade failed: %v", err)
    return
}`} />
      </>}

      {/* ── Drop Policy (v1.1.0+) ── */}
      {v110 && <>
        <h3 id="hub-drop-policy" className="text-lg font-semibold text-text-heading mt-8 mb-2">Drop Policy</h3>
        <p className="text-text-muted mb-3">
          Control what happens when a client's send buffer is full:
        </p>
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left">
                <th className="py-2 pr-4 text-text-heading font-semibold">Policy</th>
                <th className="py-2 text-text-heading font-semibold">Description</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">DropNewest</td><td className="py-2 text-text-muted">Discard the new message when buffer is full (default)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">DropOldest</td><td className="py-2 text-text-muted">Evict the oldest queued message to make room for the new one</td></tr>
            </tbody>
          </table>
        </div>
        <CodeBlock code={`// Keep the most recent data (good for real-time state updates)
hub := wshub.NewHub(
    wshub.WithDropPolicy(wshub.DropOldest),
    wshub.WithHooks(wshub.Hooks{
        OnSendDropped: func(client *wshub.Client, data []byte) {
            log.Printf("Dropped message for slow client %s", client.ID)
        },
    }),
)`} />
      </>}

      {/* ── Graceful Drain (v1.2.0+) ── */}
      {v120 && <>
        <h3 id="hub-drain" className="text-lg font-semibold text-text-heading mt-8 mb-2">Graceful Drain</h3>
        <p className="text-text-muted mb-3">
          Drain stops accepting new connections (HTTP 503) while letting existing connections
          finish in-flight messages. Designed for zero-downtime rolling deploys (Kubernetes <code className="text-accent">preStop</code>, SIGTERM handlers).
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
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Drain(ctx)</td><td className="py-2 text-text-muted">Stop accepting connections and wait for existing ones to disconnect (or context to expire)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">State()</td><td className="py-2 text-text-muted">Returns current HubState (StateRunning, StateDraining, or StateStopped)</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">IsRunning()</td><td className="py-2 text-text-muted">True when the hub is accepting new connections</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">IsDraining()</td><td className="py-2 text-text-muted">True when the hub is draining (no new connections, existing ones finishing)</td></tr>
            </tbody>
          </table>
        </div>
        <p className="text-text-muted mb-3">
          During drain, idle connections whose send buffers have been empty for the drain timeout
          (default 30s, configurable via <code className="text-accent">WithDrainTimeout</code>) are proactively closed
          with <code className="text-accent">CloseGoingAway</code> (1001).
        </p>
        <CodeBlock code={`// Kubernetes preStop / SIGTERM handler
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

hub.Drain(ctx)    // stop new connections, wait for existing ones
hub.Shutdown(ctx) // force-close anything remaining`} />

        <h4 className="text-base font-semibold text-text-heading mt-6 mb-2">Hub State</h4>
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left">
                <th className="py-2 pr-4 text-text-heading font-semibold">State</th>
                <th className="py-2 text-text-heading font-semibold">Description</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">StateRunning</td><td className="py-2 text-text-muted">Accepting new connections and processing messages normally</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">StateDraining</td><td className="py-2 text-text-muted">No new connections (HTTP 503), existing connections finishing in-flight messages</td></tr>
              <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">StateStopped</td><td className="py-2 text-text-muted">Hub shut down, all connections closed, Run loop exited</td></tr>
            </tbody>
          </table>
        </div>
        <CodeBlock code={`// Health/readiness probes
func readinessHandler(w http.ResponseWriter, r *http.Request) {
    if hub.IsRunning() {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

// Check state directly
fmt.Println(hub.State()) // "running", "draining", or "stopped"`} />
      </>}

      {/* ── Graceful Shutdown ── */}
      <h3 id="hub-shutdown" className="text-lg font-semibold text-text-heading mt-8 mb-2">Graceful Shutdown</h3>
      <p className="text-text-muted mb-3">
        The hub supports context-aware graceful shutdown that closes all client connections:
      </p>
      {v120 ? (
        <CodeBlock code={`ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Two-phase shutdown for zero-downtime deploys:
// 1. Drain — stop new connections, let existing ones finish
if err := hub.Drain(ctx); err != nil {
    log.Printf("Drain timeout: %v (forcing shutdown)", err)
}
// 2. Shutdown — force-close anything remaining
if err := hub.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}`} />
      ) : (
        <CodeBlock code={`ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Gracefully shutdown the hub — closes all connections
if err := hub.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}`} />
      )}
    </ModuleSection>
  );
}
