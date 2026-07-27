import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';
import { useVersion } from '../hooks/useVersion';

export default function AdapterDocs() {
  const { minVersion } = useVersion();
  const v160 = minVersion('v1.6.0');
  const v170 = minVersion('v1.7.0');
  return (
    <ModuleSection
      id="adapter"
      title="Multi-Node Adapters"
      description="Scale horizontally by relaying broadcasts and targeted sends across multiple hub instances through a shared message bus."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Pluggable Adapter interface for cross-node communication',
        'Built-in Redis and NATS adapter implementations',
        'Automatic deduplication via node IDs',
        'All broadcast and send methods relay transparently',
        'Local delivery is never blocked by adapter failures',
        ...(v160 ? ['Observable unmarshal failures on the subscribe path'] : []),
        ...(v170 ? ['Deterministic teardown — Close waits for subscriber goroutines'] : []),
      ]}
    >
      {/* ── Adapter Interface ── */}
      <h3 id="adapter-interface" className="text-lg font-semibold text-text-heading mt-8 mb-2">Adapter Interface</h3>
      <p className="text-text-muted mb-3">
        Implement the Adapter interface to integrate with any message bus:
      </p>
      <CodeBlock code={`type Adapter interface {
    // Publish sends a message to all other nodes.
    Publish(ctx context.Context, msg AdapterMessage) error

    // Subscribe begins receiving messages from other nodes.
    // Must not block — spawn goroutines internally.
    Subscribe(ctx context.Context, handler func(AdapterMessage)) error

    // Close shuts down the adapter, releasing all resources.
    Close() error
}`} />

      {/* ── Adapter Message ── */}
      <h3 id="adapter-message" className="text-lg font-semibold text-text-heading mt-8 mb-2">Adapter Message</h3>
      <p className="text-text-muted mb-3">
        The wire format used for inter-node communication:
      </p>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Field</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">NodeID</td><td className="py-2 text-text-muted">Originating hub node (used for deduplication)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Type</td><td className="py-2 text-text-muted">Operation type (broadcast, room, user, client, presence)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Room</td><td className="py-2 text-text-muted">Target room name (for room-scoped operations)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">UserID</td><td className="py-2 text-text-muted">Target user (for SendToUser)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ClientID</td><td className="py-2 text-text-muted">Target client (for SendToClient)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ExceptClientIDs</td><td className="py-2 text-text-muted">Client IDs to exclude from delivery</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">MsgType</td><td className="py-2 text-text-muted">WebSocket message type (text or binary)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Data</td><td className="py-2 text-text-muted">Raw message payload</td></tr>
          </tbody>
        </table>
      </div>
      <p className="text-text-muted mb-3">
        Supported operation types:
      </p>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Constant</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterBroadcast</td><td className="py-2 text-text-muted">Broadcast to all clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterBroadcastExcept</td><td className="py-2 text-text-muted">Broadcast excluding specific clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterRoom</td><td className="py-2 text-text-muted">Broadcast to a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterRoomExcept</td><td className="py-2 text-text-muted">Broadcast to a room excluding specific clients</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterUser</td><td className="py-2 text-text-muted">Send to all connections of a user</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterClient</td><td className="py-2 text-text-muted">Send to a specific client</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AdapterPresence</td><td className="py-2 text-text-muted">Presence heartbeat</td></tr>
          </tbody>
        </table>
      </div>

      {/* ── Redis Adapter ── */}
      <h3 id="adapter-redis" className="text-lg font-semibold text-text-heading mt-8 mb-2">Redis Adapter</h3>
      <p className="text-text-muted mb-3">
        Uses Redis Pub/Sub for cross-node communication. Install the adapter module:
      </p>
      <CodeBlock lang="bash" code="go get github.com/KARTIKrocks/wshub/adapter/redis" />
      <CodeBlock code={`import (
    "github.com/KARTIKrocks/wshub"
    wshubredis "github.com/KARTIKrocks/wshub/adapter/redis"
    goredis "github.com/redis/go-redis/v9"
)

rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})

adapter := wshubredis.New(rdb,
    wshubredis.WithChannel("myapp:wshub"), // default: "wshub:messages"
)

hub := wshub.NewHub(
    wshub.WithAdapter(adapter),
    wshub.WithNodeID("node-1"), // optional: stable ID for debugging
)
go hub.Run()`} />

      {/* ── NATS Adapter ── */}
      <h3 id="adapter-nats" className="text-lg font-semibold text-text-heading mt-8 mb-2">NATS Adapter</h3>
      <p className="text-text-muted mb-3">
        Uses NATS core Pub/Sub for lower-latency cross-node communication. Install the adapter module:
      </p>
      <CodeBlock lang="bash" code="go get github.com/KARTIKrocks/wshub/adapter/nats" />
      <CodeBlock code={`import (
    "github.com/KARTIKrocks/wshub"
    wshubnats "github.com/KARTIKrocks/wshub/adapter/nats"
    gonats "github.com/nats-io/nats.go"
)

nc, _ := gonats.Connect("nats://localhost:4222")

adapter := wshubnats.New(nc,
    wshubnats.WithSubject("myapp.wshub"), // default: "wshub.messages"
)

hub := wshub.NewHub(
    wshub.WithAdapter(adapter),
    wshub.WithNodeID("node-1"),
)
go hub.Run()`} />

      {/* ── Adapter Options ── */}
      {v160 && (
        <>
          <h3 id="adapter-options" className="text-lg font-semibold text-text-heading mt-8 mb-2">Adapter Options</h3>
          <div className="overflow-x-auto mb-4">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left">
                  <th className="py-2 pr-4 text-text-heading font-semibold">Option</th>
                  <th className="py-2 text-text-heading font-semibold">Description</th>
                </tr>
              </thead>
              <tbody>
                <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithChannel(name)</td><td className="py-2 text-text-muted">Redis only — Pub/Sub channel. Default <span className="font-mono">wshub:messages</span></td></tr>
                <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithSubject(name)</td><td className="py-2 text-text-muted">NATS only — subject. Default <span className="font-mono">wshub.messages</span></td></tr>
                <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">WithUnmarshalErrorHandler(fn)</td><td className="py-2 text-text-muted">Observe messages that fail JSON unmarshaling in the subscribe path</td></tr>
              </tbody>
            </table>
          </div>
          <p className="text-text-muted mb-3">
            Added in v1.6.0. A malformed payload on the bus — a stray publisher, a
            partial write, a version mismatch between nodes — is dropped by the subscribe
            path. Without a handler that happens silently, so a node quietly stops relaying
            with nothing in the logs. Both adapters accept the same option:
          </p>
          <CodeBlock code={`adapter := wshubredis.New(rdb,
    wshubredis.WithUnmarshalErrorHandler(func(data []byte, err error) {
        log.Printf("wshub: dropping malformed adapter message: %v (%d bytes)", err, len(data))
    }),
)

// Identical on the NATS adapter:
adapter := wshubnats.New(nc,
    wshubnats.WithUnmarshalErrorHandler(func(data []byte, err error) {
        log.Printf("wshub: dropping malformed adapter message: %v (%d bytes)", err, len(data))
    }),
)`} />
        </>
      )}

      {/* ── How It Works ── */}
      <h3 id="adapter-how" className="text-lg font-semibold text-text-heading mt-8 mb-2">How It Works</h3>
      <p className="text-text-muted mb-3">
        When an adapter is configured, every public broadcast and send method delivers locally first,
        then publishes to the adapter so other nodes can relay the message to their clients.
        Messages originating from the current node are automatically deduplicated via the node ID.
      </p>
      <CodeBlock code={`// These all work transparently across nodes:
hub.Broadcast([]byte("hello everyone"))          // all nodes
hub.BroadcastToRoom("chat", []byte("hi room"))   // room members on all nodes
hub.SendToUser("user-123", []byte("hi"))          // user's connections on all nodes
hub.SendToClient(clientID, []byte("hi"))          // finds client across nodes

// Shutdown closes the adapter before waiting on goroutines
hub.Shutdown(ctx)`} />

      {/* ── Lifecycle ── */}
      {v170 && (
        <>
          <h3 id="adapter-lifecycle" className="text-lg font-semibold text-text-heading mt-8 mb-2">Lifecycle</h3>
          <p className="text-text-muted mb-3">
            The bundled adapters guarantee the following teardown behaviour as of{' '}
            <span className="font-mono text-accent">adapter/redis v0.2.2</span> and{' '}
            <span className="font-mono text-accent">adapter/nats v0.2.2</span>. Earlier
            releases could deadlock in <span className="font-mono text-accent">Close</span>{' '}
            after a <span className="font-mono text-accent">Subscribe</span>, so upgrade both
            alongside wshub v1.7.0.
          </p>
          <ul className="text-text-muted mb-4 list-disc pl-5 space-y-1">
            <li>
              <span className="font-mono text-accent">Close</span> waits for the subscriber
              goroutine to finish, so no delivery is in flight once it returns.
            </li>
            <li>
              <span className="font-mono text-accent">Close</span> is idempotent — calling it
              twice returns <span className="font-mono text-accent">nil</span> the second time.
              <span className="font-mono text-accent"> Hub.Shutdown</span> already closes a
              configured adapter, so an explicit call is only needed for an adapter you own
              outside a hub.
            </li>
            <li>
              After <span className="font-mono text-accent">Close</span>, both{' '}
              <span className="font-mono text-accent">Publish</span> and{' '}
              <span className="font-mono text-accent">Subscribe</span> return{' '}
              <span className="font-mono text-accent">ErrClosed</span> rather than silently
              doing nothing.
            </li>
            <li>
              Calling <span className="font-mono text-accent">Subscribe</span> again replaces
              the previous subscription and releases its goroutine.
            </li>
            <li>
              Cancelling the context passed to <span className="font-mono text-accent">Subscribe</span>{' '}
              stops delivery.
            </li>
          </ul>
          <CodeBlock code={`// A hub-owned adapter needs no explicit Close — Shutdown does it:
hub.Shutdown(ctx)

// A standalone adapter is yours to close, before the underlying
// client goes away:
adapter := wshubredis.New(rdb)
defer rdb.Close()
defer adapter.Close() // runs first: waits for the subscriber goroutine`} />
        </>
      )}
    </ModuleSection>
  );
}
