import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function ErrorsDocs() {
  return (
    <ModuleSection
      id="errors"
      title="Errors"
      description="Comprehensive sentinel errors for connection, client, room, and limit error handling."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Sentinel errors for all error categories',
        'Compatible with errors.Is for matching',
        'Clear error categories: connection, client, room, limits, auth',
      ]}
    >
      {/* ── Connection Errors ── */}
      <h3 id="errors-connection" className="text-lg font-semibold text-text-heading mt-8 mb-2">Connection Errors</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Error</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrConnectionClosed</td><td className="py-2 text-text-muted">Connection is already closed</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrWriteTimeout</td><td className="py-2 text-text-muted">Write operation timed out</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrReadTimeout</td><td className="py-2 text-text-muted">Read operation timed out</td></tr>
          </tbody>
        </table>
      </div>

      {/* ── Client Errors ── */}
      <h3 id="errors-client" className="text-lg font-semibold text-text-heading mt-8 mb-2">Client Errors</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Error</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrClientNotFound</td><td className="py-2 text-text-muted">Client with given ID not found</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrClientAlreadyExists</td><td className="py-2 text-text-muted">Client with given ID already registered</td></tr>
          </tbody>
        </table>
      </div>

      {/* ── Room Errors ── */}
      <h3 id="errors-room" className="text-lg font-semibold text-text-heading mt-8 mb-2">Room Errors</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Error</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrEmptyRoomName</td><td className="py-2 text-text-muted">Room name cannot be empty</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrRoomNotFound</td><td className="py-2 text-text-muted">Room does not exist</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrAlreadyInRoom</td><td className="py-2 text-text-muted">Client is already in the room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrNotInRoom</td><td className="py-2 text-text-muted">Client is not in the room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrRoomFull</td><td className="py-2 text-text-muted">Room has reached max capacity</td></tr>
          </tbody>
        </table>
      </div>

      {/* ── Limit Errors ── */}
      <h3 id="errors-limits" className="text-lg font-semibold text-text-heading mt-8 mb-2">Limit Errors</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Error</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrMaxConnectionsReached</td><td className="py-2 text-text-muted">Hub has reached maximum connections</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrMaxUserConnectionsReached</td><td className="py-2 text-text-muted">User has reached max connections per user</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrMaxRoomsReached</td><td className="py-2 text-text-muted">Client has reached max rooms per client</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrRateLimitExceeded</td><td className="py-2 text-text-muted">Client exceeded message rate limit</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrAuthenticationFailed</td><td className="py-2 text-text-muted">Authentication failed</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">ErrUnauthorized</td><td className="py-2 text-text-muted">Client is not authorized</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Check errors with errors.Is
if errors.Is(err, wshub.ErrConnectionClosed) {
    log.Println("Connection already closed")
}

if errors.Is(err, wshub.ErrRateLimitExceeded) {
    log.Println("Client sending too fast")
}

if errors.Is(err, wshub.ErrRoomFull) {
    client.SendText("Room is full, try again later")
}

if errors.Is(err, wshub.ErrMaxConnectionsReached) {
    log.Println("Server at capacity")
}`} />
    </ModuleSection>
  );
}
