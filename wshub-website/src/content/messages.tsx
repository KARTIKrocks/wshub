import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function MessagesDocs() {
  return (
    <ModuleSection
      id="messages"
      title="Messages"
      description="The Message type represents incoming WebSocket messages with helpers for common formats."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Typed message representation (text and binary)',
        'Convenience helpers for text and JSON parsing',
        'Includes sender client ID and receive timestamp',
      ]}
    >
      {/* ── Message Type ── */}
      <h3 id="messages-type" className="text-lg font-semibold text-text-heading mt-8 mb-2">Message Type</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Field / Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Type</td><td className="py-2 text-text-muted">MessageType (TextMessage or BinaryMessage)</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Data</td><td className="py-2 text-text-muted">Raw message data as []byte</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ClientID</td><td className="py-2 text-text-muted">Sender's client ID</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Time</td><td className="py-2 text-text-muted">Receive timestamp</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">Text()</td><td className="py-2 text-text-muted">Data as string</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">JSON(v)</td><td className="py-2 text-text-muted">Unmarshal data as JSON into v</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`type Message struct {
    Type     MessageType // TextMessage, BinaryMessage
    Data     []byte      // Raw message data
    ClientID string      // Sender's client ID
    Time     time.Time   // Receive timestamp
}

// Convenience helpers
text := msg.Text()         // Data as string

var payload ChatMessage
err := msg.JSON(&payload)  // Unmarshal as JSON`} />

      {/* ── Message Handler ── */}
      <h3 id="messages-handler" className="text-lg font-semibold text-text-heading mt-8 mb-2">Message Handler</h3>
      <p className="text-text-muted mb-3">
        Set a message handler when creating the hub to process incoming messages:
      </p>
      <CodeBlock code={`hub := wshub.NewHub(
    wshub.WithMessageHandler(func(client *wshub.Client, msg *wshub.Message) error {
        // Parse the incoming message
        var chatMsg struct {
            Room string \`json:"room"\`
            Text string \`json:"text"\`
        }
        if err := msg.JSON(&chatMsg); err != nil {
            return err
        }

        // Broadcast to a room
        response, _ := json.Marshal(map[string]string{
            "from": client.ID,
            "text": chatMsg.Text,
        })
        hub.BroadcastToRoom(chatMsg.Room, response)
        return nil
    }),
)`} />
    </ModuleSection>
  );
}
