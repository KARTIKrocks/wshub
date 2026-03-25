import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';
import { useVersion } from '../hooks/useVersion';

export default function ClientDocs() {
  const { minVersion } = useVersion();
  const v110 = minVersion('v1.1.0');

  return (
    <ModuleSection
      id="client"
      title="Client"
      description="Represents a single WebSocket connection with methods for sending messages, managing metadata, and handling events."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Unique UUID-based client identification',
        'User ID support for multi-device connections',
        'Arbitrary per-client metadata storage',
        'Multiple message sending formats (text, binary, JSON)',
        'Per-client event callbacks (OnMessage, OnClose, OnError)',
        'Access to the original HTTP request',
      ]}
    >
      {/* ── Properties ── */}
      <h3 id="client-properties" className="text-lg font-semibold text-text-heading mt-8 mb-2">Properties</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Property / Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">client.ID</td><td className="py-2 text-text-muted">Unique client identifier (UUID)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SetUserID(userID)</td><td className="py-2 text-text-muted">Set user ID for multi-device support</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">GetUserID()</td><td className="py-2 text-text-muted">Get the assigned user ID</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ConnectedAt()</td><td className="py-2 text-text-muted">Connection timestamp</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">IsClosed()</td><td className="py-2 text-text-muted">Whether the connection is closed</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ClosedAt()</td><td className="py-2 text-text-muted">When the connection was closed</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Request()</td><td className="py-2 text-text-muted">Access the original HTTP request</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Access client properties
log.Printf("Client ID: %s", client.ID)
log.Printf("Connected at: %v", client.ConnectedAt())

// Multi-device user identification
client.SetUserID("user-456")
userID := client.GetUserID()

// Access original HTTP request (for auth headers, cookies, etc.)
req := client.Request()
token := req.Header.Get("Authorization")`} />

      {/* ── Sending Messages ── */}
      <h3 id="client-sending" className="text-lg font-semibold text-text-heading mt-8 mb-2">Sending Messages</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Send(data)</td><td className="py-2 text-text-muted">Send raw bytes</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendText(text)</td><td className="py-2 text-text-muted">Send a text string</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendJSON(v)</td><td className="py-2 text-text-muted">JSON-encode and send</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendBinary(data)</td><td className="py-2 text-text-muted">Send binary message</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendMessage(msgType, data)</td><td className="py-2 text-text-muted">Send with specific message type{v110 ? ' (applies drop policy)' : ''}</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendWithContext(ctx, data)</td><td className="py-2 text-text-muted">Send text with context support{v110 ? ' (blocks until enqueued)' : ''}</td></tr>
            {v110 && <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">SendMessageWithContext(ctx, msgType, data)</td><td className="py-2 text-text-muted">Send with type and context (blocks until enqueued)</td></tr>}
          </tbody>
        </table>
      </div>
      {v110 ? (
        <CodeBlock code={`// Send different message types
client.Send([]byte("raw bytes"))
client.SendText("hello")
client.SendBinary(binaryData)

// Send JSON
client.SendJSON(map[string]any{
    "type": "chat",
    "text": "hello world",
    "time": time.Now(),
})

// Send with context (blocks until enqueued, unlike SendMessage which applies drop policy)
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
client.SendWithContext(ctx, []byte("important message"))

// Send with specific type and context
client.SendMessageWithContext(ctx, wshub.BinaryMessage, binaryData)`} />
      ) : (
        <CodeBlock code={`// Send different message types
client.Send([]byte("raw bytes"))
client.SendText("hello")
client.SendBinary(binaryData)

// Send JSON
client.SendJSON(map[string]any{
    "type": "chat",
    "text": "hello world",
    "time": time.Now(),
})

// Send with context (for timeout control)
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
client.SendWithContext(ctx, []byte("important message"))`} />
      )}

      {/* ── Metadata ── */}
      <h3 id="client-metadata" className="text-lg font-semibold text-text-heading mt-8 mb-2">Metadata</h3>
      <p className="text-text-muted mb-3">
        Store arbitrary per-client data using the metadata API:
      </p>
      <CodeBlock code={`// Store request-scoped data on the client
client.SetMetadata("role", "admin")
client.SetMetadata("display_name", "Alice")

// Retrieve metadata
role, ok := client.GetMetadata("role")
if ok {
    log.Printf("Role: %v", role)
}

// Remove metadata
client.DeleteMetadata("temporary_key")`} />

      {/* ── Callbacks ── */}
      <h3 id="client-callbacks" className="text-lg font-semibold text-text-heading mt-8 mb-2">Callbacks</h3>
      <p className="text-text-muted mb-3">
        Register per-client event handlers:
      </p>
      <CodeBlock code={`client.OnMessage(func(c *wshub.Client, msg *wshub.Message) {
    // Handle messages for this specific client
    log.Printf("Message: %s", msg.Text())
})

client.OnClose(func(c *wshub.Client) {
    // Clean up when this client disconnects
    log.Printf("Client %s disconnected", c.ID)
})

client.OnError(func(c *wshub.Client, err error) {
    // Handle errors for this client
    log.Printf("Error for %s: %v", c.ID, err)
})`} />

      {/* ── Closing ── */}
      <h3 id="client-closing" className="text-lg font-semibold text-text-heading mt-8 mb-2">Closing</h3>
      <CodeBlock code={`// Close with default close code
client.Close()

// Close with a specific WebSocket close code and reason
client.CloseWithCode(websocket.CloseNormalClosure, "goodbye")`} />
    </ModuleSection>
  );
}
