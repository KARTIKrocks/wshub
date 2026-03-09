import CodeBlock from '../components/CodeBlock';
import ModuleSection from '../components/ModuleSection';

export default function RoomsDocs() {
  return (
    <ModuleSection
      id="rooms"
      title="Rooms"
      description="Group clients into rooms for targeted broadcasting. Rooms are lazily created on first join and automatically cleaned up when empty."
      importPath="github.com/KARTIKrocks/wshub"
      features={[
        'Lazy room creation — rooms appear on first join',
        'Automatic cleanup — rooms removed when empty',
        'Per-room locks for minimal contention',
        'Room-scoped broadcasting',
        'Client-side room queries',
      ]}
    >
      {/* ── Joining & Leaving ── */}
      <h3 id="rooms-joining" className="text-lg font-semibold text-text-heading mt-8 mb-2">Joining & Leaving</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.JoinRoom(client, room)</td><td className="py-2 text-text-muted">Add client to a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.LeaveRoom(client, room)</td><td className="py-2 text-text-muted">Remove client from a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.LeaveAllRooms(client)</td><td className="py-2 text-text-muted">Remove client from all rooms</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Join a room (created lazily if it doesn't exist)
err := hub.JoinRoom(client, "chat-general")
err := hub.JoinRoom(client, "notifications")

// Leave a specific room
err := hub.LeaveRoom(client, "chat-general")

// Leave all rooms at once
hub.LeaveAllRooms(client)`} />

      {/* ── Room Broadcasting ── */}
      <h3 id="rooms-broadcasting" className="text-lg font-semibold text-text-heading mt-8 mb-2">Room Broadcasting</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastToRoom(room, data)</td><td className="py-2 text-text-muted">Send to all clients in a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">BroadcastToRoomExcept(room, data, except...)</td><td className="py-2 text-text-muted">Send to room except specific clients</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Broadcast to everyone in a room
hub.BroadcastToRoom("chat-general", []byte("hello room!"))

// Broadcast to room except the sender
hub.BroadcastToRoomExcept("chat-general", []byte(msg), sender)`} />

      {/* ── Querying Rooms ── */}
      <h3 id="rooms-querying" className="text-lg font-semibold text-text-heading mt-8 mb-2">Querying Rooms</h3>
      <div className="overflow-x-auto mb-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="py-2 pr-4 text-text-heading font-semibold">Method</th>
              <th className="py-2 text-text-heading font-semibold">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.RoomClients(room)</td><td className="py-2 text-text-muted">Get all clients in a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.RoomCount(room)</td><td className="py-2 text-text-muted">Count clients in a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.RoomNames()</td><td className="py-2 text-text-muted">Get all room names</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">hub.RoomExists(room)</td><td className="py-2 text-text-muted">Check if a room exists</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">client.Rooms()</td><td className="py-2 text-text-muted">List client's rooms</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">client.InRoom(name)</td><td className="py-2 text-text-muted">Check if client is in a room</td></tr>
            <tr className="border-b border-border/50"><td className="py-2 pr-4 font-mono text-accent whitespace-nowrap">client.RoomCount()</td><td className="py-2 text-text-muted">Number of rooms the client is in</td></tr>
          </tbody>
        </table>
      </div>
      <CodeBlock code={`// Hub-level room queries
clients := hub.RoomClients("chat-general")
count := hub.RoomCount("chat-general")
rooms := hub.RoomNames()
exists := hub.RoomExists("chat-general")

// Client-level room queries
myRooms := client.Rooms()
inRoom := client.InRoom("chat-general")
roomCount := client.RoomCount()`} />
    </ModuleSection>
  );
}
