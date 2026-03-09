import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function HooksDocs() {
  return (
    <ModuleSection
      id="hooks"
      title="Hooks"
      description="Lifecycle hooks let you run custom logic at key connection, message, and room events."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Connection lifecycle: BeforeConnect, AfterConnect, BeforeDisconnect, AfterDisconnect',
        'Message lifecycle: BeforeMessage, AfterMessage',
        'Room lifecycle: BeforeRoomJoin, AfterRoomJoin, BeforeRoomLeave, AfterRoomLeave',
        'Error handling hook: OnError',
        'Before hooks can reject operations by returning an error',
      ]}
    >
      {/* ── Connection Hooks ── */}
      <h3 id="hooks-connection" className="text-lg font-semibold text-text-heading mt-8 mb-2">Connection Hooks</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Hook</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BeforeConnect(r *http.Request) error</td><td className="py-2 text-text-muted">Called before upgrading — return error to reject</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AfterConnect(client *Client)</td><td className="py-2 text-text-muted">Called after a client connects</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BeforeDisconnect(client *Client)</td><td className="py-2 text-text-muted">Called before a client disconnects</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AfterDisconnect(client *Client)</td><td className="py-2 text-text-muted">Called after a client disconnects</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">OnError(client *Client, err error)</td><td className="py-2 text-text-muted">Called on client errors</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`hub := wshub.NewHub(
    wshub.WithHooks(wshub.Hooks{
        BeforeConnect: func(r *http.Request) error {
            // Validate auth token before upgrade
            token := r.Header.Get("Authorization")
            if !validateToken(token) {
                return wshub.ErrAuthenticationFailed
            }
            return nil
        },
        AfterConnect: func(client *wshub.Client) {
            // Set user ID from auth context
            userID := extractUserID(client.Request())
            client.SetUserID(userID)
            log.Printf("User %s connected (client: %s)", userID, client.ID)
        },
        AfterDisconnect: func(client *wshub.Client) {
            log.Printf("Client %s disconnected", client.ID)
        },
        OnError: func(client *wshub.Client, err error) {
            log.Printf("Error for %s: %v", client.ID, err)
        },
    }),
)`} />

      {/* ── Message Hooks ── */}
      <h3 id="hooks-message" className="text-lg font-semibold text-text-heading mt-8 mb-2">Message Hooks</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Hook</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BeforeMessage(client, msg) (*Message, error)</td><td className="py-2 text-text-muted">Called before processing — can modify or reject the message</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AfterMessage(client, msg, err)</td><td className="py-2 text-text-muted">Called after message processing completes</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`wshub.WithHooks(wshub.Hooks{
    BeforeMessage: func(client *wshub.Client, msg *wshub.Message) (*wshub.Message, error) {
        // Reject empty messages
        if len(msg.Data) == 0 {
            return nil, fmt.Errorf("empty message")
        }
        // Modify the message (e.g., sanitize)
        return msg, nil
    },
    AfterMessage: func(client *wshub.Client, msg *wshub.Message, err error) {
        if err != nil {
            log.Printf("Message handling failed: %v", err)
        }
    },
})`} />

      {/* ── Room Hooks ── */}
      <h3 id="hooks-room" className="text-lg font-semibold text-text-heading mt-8 mb-2">Room Hooks</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Hook</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BeforeRoomJoin(client, room) error</td><td className="py-2 text-text-muted">Called before joining — return error to reject</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AfterRoomJoin(client, room)</td><td className="py-2 text-text-muted">Called after joining a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BeforeRoomLeave(client, room)</td><td className="py-2 text-text-muted">Called before leaving a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">AfterRoomLeave(client, room)</td><td className="py-2 text-text-muted">Called after leaving a room</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`wshub.WithHooks(wshub.Hooks{
    BeforeRoomJoin: func(client *wshub.Client, room string) error {
        // Check if client is authorized to join this room
        if !isAuthorized(client, room) {
            return wshub.ErrUnauthorized
        }
        return nil
    },
    AfterRoomJoin: func(client *wshub.Client, room string) {
        // Notify room members
        hub.BroadcastToRoomExcept(room, []byte(client.ID+" joined"), client)
    },
    AfterRoomLeave: func(client *wshub.Client, room string) {
        hub.BroadcastToRoom(room, []byte(client.ID+" left"))
    },
})`} />
    </ModuleSection>
  );
}
