---
id: errors
title: Errors
description: Sentinel errors for connection, hub state, client, room, and limit error handling.
---

# Errors

Comprehensive sentinel errors for connection, client, room, and limit error
handling.

```go
import "github.com/KARTIKrocks/wshub"
```

- Sentinel errors for all error categories
- Compatible with `errors.Is` for matching
- Clear error categories: connection, client, room, limits, auth

> Exact signatures live on
> [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub#pkg-variables).

## Connection Errors

| Error | Description |
| --- | --- |
| `ErrConnectionClosed` | Connection is already closed |
| `ErrInvalidMessage` | Invalid message received |
| `ErrSendBufferFull` | Client's send buffer is full (returned by `SendMessage`) |

## Hub State Errors

| Error | Description |
| --- | --- |
| `ErrHubDraining` | `UpgradeConnection` called while the hub is draining (HTTP 503) |
| `ErrHubStopped` | `UpgradeConnection` called after the hub has been shut down (HTTP 503) |
| `ErrHubNotStarted` | `UpgradeConnection` called before `Run()` has been started (HTTP 503) |

## Client Errors

| Error | Description |
| --- | --- |
| `ErrClientNotFound` | Client with the given ID not found |

## Room Errors

| Error | Description |
| --- | --- |
| `ErrEmptyRoomName` | Room name cannot be empty |
| `ErrRoomNotFound` | Room does not exist |
| `ErrAlreadyInRoom` | Client is already in the room |
| `ErrNotInRoom` | Client is not in the room |
| `ErrRoomFull` | Room has reached max capacity |

## Limit Errors

| Error | Description |
| --- | --- |
| `ErrMaxConnectionsReached` | Hub has reached maximum connections |
| `ErrMaxUserConnectionsReached` | User has reached max connections per user |
| `ErrMaxRoomsReached` | Client has reached max rooms per client |
| `ErrRateLimitExceeded` | Client exceeded the message rate limit |
| `ErrAuthenticationFailed` | Authentication failed |
| `ErrUnauthorized` | Client is not authorized |

## Matching Errors

```go
// Check errors with errors.Is
if errors.Is(err, wshub.ErrConnectionClosed) {
    log.Println("Connection already closed")
}

if errors.Is(err, wshub.ErrSendBufferFull) {
    log.Println("Client send buffer full, message dropped")
}

if errors.Is(err, wshub.ErrRateLimitExceeded) {
    log.Println("Client sending too fast")
}

if errors.Is(err, wshub.ErrRoomFull) {
    client.SendText("Room is full, try again later")
}

if errors.Is(err, wshub.ErrMaxConnectionsReached) {
    log.Println("Server at capacity")
}
```
