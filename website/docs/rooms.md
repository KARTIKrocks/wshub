---
id: rooms
title: Rooms
description: Group clients into rooms for targeted broadcasting, with lazy creation and automatic cleanup.
---

# Rooms

Group clients into rooms for targeted broadcasting. Rooms are lazily created on
first join and automatically cleaned up when empty.

```go
import "github.com/KARTIKrocks/wshub"
```

- Lazy room creation — rooms appear on first join
- Automatic cleanup — rooms removed when empty
- Per-room locks for minimal contention
- Room-scoped broadcasting
- Client-side room queries

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub).

## Joining and Leaving

| Method | Description |
| --- | --- |
| `hub.JoinRoom(client, room)` | Add a client to a room |
| `hub.LeaveRoom(client, room)` | Remove a client from a room |
| `hub.LeaveAllRooms(client)` | Remove a client from all rooms |

```go
// Join a room (created lazily if it doesn't exist)
err := hub.JoinRoom(client, "chat-general")
err := hub.JoinRoom(client, "notifications")

// Leave a specific room
err := hub.LeaveRoom(client, "chat-general")

// Leave all rooms at once
hub.LeaveAllRooms(client)
```

## Room Broadcasting

| Method | Description |
| --- | --- |
| `BroadcastToRoom(room, data)` | Send text to all clients in a room |
| `BroadcastBinaryToRoom(room, data)` | Send binary to all clients in a room |
| `BroadcastToRoomWithContext(ctx, room, data)` | Send to a room with context (blocks until enqueued) |
| `BroadcastToRoomExcept(room, data, except...)` | Send text to a room except specific clients |
| `BroadcastBinaryToRoomExcept(room, data, except...)` | Send binary to a room except specific clients |

```go
// Broadcast to everyone in a room
hub.BroadcastToRoom("chat-general", []byte("hello room!"))

// Broadcast to room except the sender
hub.BroadcastToRoomExcept("chat-general", []byte(msg), sender)
```

## Querying Rooms

| Method | Description |
| --- | --- |
| `hub.RoomClients(room)` | Get all clients in a room |
| `hub.RoomCount(room)` | Count clients in a room (local node) |
| `hub.GlobalRoomCount(room)` | Count clients in a room across all nodes (requires [presence](./presence.md)) |
| `hub.RoomNames()` | Get all room names |
| `hub.RoomExists(room)` | Check whether a room exists |
| `client.Rooms()` | List the client's rooms |
| `client.InRoom(name)` | Check whether the client is in a room |
| `client.RoomCount()` | Number of rooms the client is in |

```go
// Hub-level room queries
clients := hub.RoomClients("chat-general")
count := hub.RoomCount("chat-general")
rooms := hub.RoomNames()
exists := hub.RoomExists("chat-general")

// Client-level room queries
myRooms := client.Rooms()
inRoom := client.InRoom("chat-general")
roomCount := client.RoomCount()
```
