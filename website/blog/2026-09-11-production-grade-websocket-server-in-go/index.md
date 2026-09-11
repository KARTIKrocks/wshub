---
slug: production-grade-websocket-server-in-go
title: Building a Production-Grade WebSocket Server in Go
authors: [kartik]
tags: [go, websocket, backend, distributed-systems]
description: What actually breaks when a `gorilla/websocket` demo meets production traffic, and the eight problems you have to solve — in order — to fix it.
---

# Building a Production-Grade WebSocket Server in Go

Every Go WebSocket server starts the same way. You pull in `gorilla/websocket`
or `nhooyr.io/websocket`, write an `Upgrade` call, spin up a goroutine that
reads frames in a loop, and it works. You broadcast a message by iterating a
`map[*Client]bool` and calling `conn.WriteMessage` on each one. Twenty lines,
a working demo, and a reasonable belief that the hard part is behind you.

It isn't. The hard part starts at the traffic your demo never saw: a client
that stops reading, a deploy that needs to happen without dropping
connections, a second replica that has no idea the first one exists. None of
that is in the WebSocket RFC, and none of it is optional once real users are
on the other end of the socket.

{/* truncate */}

This post is the list of problems I hit, in the order production traffic
surfaces them, and what each one actually requires — not "add a mutex," but
the specific mechanism and the tradeoff it forces. I ended up packaging the
result as an open-source library, [wshub](https://github.com/KARTIKrocks/wshub),
but the problems here apply whether you use it or write your own.

## 1. The connection lifecycle is two goroutines, not one

A `net.Conn` is full-duplex, but `gorilla/websocket`'s `*Conn` is not
safe for concurrent writes from multiple goroutines — only one writer at a
time, ever. That single constraint decides the shape of every connection you
manage:

- **One `readPump` goroutine** blocks on `conn.ReadMessage()` and feeds
  inbound frames to your handler.
- **One `writePump` goroutine** owns the connection exclusively for writes,
  draining a per-client buffered channel (`chan []byte`) and calling
  `conn.WriteMessage` itself.

Every other part of the system — your HTTP handlers, your business logic,
another goroutine reacting to a Redis message — writes to that channel and
never touches the socket directly. That single rule (only the writePump
writes) is what makes the rest of this list tractable. Skip it and you'll
eventually get a data race the race detector won't catch in dev, because it
only reproduces under concurrent load.

The cost: three long-lived goroutines per connection (readPump, writePump,
plus whatever handles the HTTP upgrade), each with its own stack. At 10,000
connections that's the dominant driver of memory per connection — roughly
25–27 KB in practice, most of it goroutine stacks and channel buffers, not the
socket itself.

## 2. Concurrent access to the client map

The hub needs a `map[string]*Client` for O(1) lookup and a way to iterate it
for broadcast — and both happen from different goroutines constantly.
`sync.RWMutex` is the obvious answer, but the naive version has a trap:

```go
// Don't do this — the lock is held for the entire broadcast
func (h *Hub) Broadcast(data []byte) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    for _, c := range h.clients {
        c.send <- data // blocks here if a client's channel is full
    }
}
```

If one client's `send` channel is full, this blocks the whole broadcast — and
since it's holding an `RLock`, it blocks every *other* goroutine trying to
register or deregister a client, too. One slow client now stalls the entire
hub. The fix is to take a **snapshot** of the client slice under the lock,
then release the lock before touching any channel:

```go
func (h *Hub) Broadcast(data []byte) {
    h.mu.RLock()
    snapshot := h.clientSnapshot() // copy the slice, still under RLock
    h.mu.RUnlock()

    for _, c := range snapshot {
        c.trySend(data) // non-blocking; see backpressure below
    }
}
```

This is the difference between a broadcast that's O(1) allocations and lock
contention that scales with client count, versus one that's actually
lock-free for the read path. Measured with mock clients: broadcasting to
100,000 clients takes ~22.6 ms with zero allocations; to 1,000,000 clients,
~269 ms, still zero allocations. Targeted sends (`SendToClient`) are O(1)
regardless of hub size — ~105 ns whether there are 100K or 1M clients on the
hub, because they go straight through the map, never through the snapshot.

## 3. Broadcast is not one primitive — it's four

"Send this to everyone" is the easy case. Production traffic needs:

- **Broadcast** — everyone.
- **Broadcast to a room** — a scoped subset (a chat channel, a game
  instance, a document's collaborators).
- **Broadcast except** — everyone but the sender, so you don't echo a user's
  own message back to them.
- **Send to user** — not a connection, a *user*, who may have three tabs
  open and three separate `*Client`s. This needs its own index
  (`map[userID][]*Client`) alongside the connection map, because a user
  reconnecting on a fourth device shouldn't require touching every existing
  connection's metadata.

Each of these has to preserve the same non-blocking guarantee as plain
broadcast. `BroadcastExcept` in particular is tempting to implement as
"broadcast, then skip in the loop" — which is exactly what you want, as long
as the exclusion check doesn't force an allocation per call. Done right, it
costs one allocation per call (for the exclusion set), not per client.

## 4. Backpressure: the problem nobody's demo has

Here's the scenario every WebSocket server eventually hits: one client is on
a flaky mobile connection, or its own event loop is blocked, or it just
stopped reading. The server keeps trying to write. What happens?

If you write directly to the socket, that goroutine now blocks on a
platform TCP buffer that never drains, and depending on how you built the
send path, this can propagate all the way back to the broadcast loop — the
"one slow client stalls everyone" bug again, this time in the OS, not your
mutex.

The mitigation is a **bounded per-client channel with an explicit drop
policy** when it's full:

```go
func (c *Client) trySend(data []byte) {
    select {
    case c.send <- data:
    default:
        // buffer full — apply policy instead of blocking
        c.applyDropPolicy(data)
    }
}
```

There are two reasonable policies, and the right one depends on what the
message *means*:

- **Drop newest** (default) — discard the message that just arrived. Correct
  for append-only streams like chat, where losing the latest of several
  queued messages is recoverable.
- **Drop oldest** — evict the head of the queue to make room. Correct for
  state-replacement streams — a live cursor position, a stock ticker, a
  presence indicator — where only the *latest* value matters and an old one
  is actively wrong to deliver.

Either way, silently dropping data is a bug waiting to be discovered in
production unless it's observable. A drop hook —
`OnSendDropped(client, data)` — that increments a metric or logs the client
ID is not optional; it's the difference between "we tuned the buffer size
after seeing drop-rate alerts" and "we found out during a user complaint
three weeks later."

## 5. Graceful shutdown is two phases, not one

`os.Signal` handling for `SIGTERM` is the easy half. The question that
matters is: **what happens to the 8,000 connections currently open when the
pod gets killed for a routine deploy?**

Closing them all immediately is a correctness bug disguised as a shutdown
routine — every one of those clients gets a hard disconnect and has to
reconnect, all at once, which is its own thundering-herd problem against
whatever server picks them up next.

The pattern that actually works is two distinct phases:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

hub.Drain(ctx)    // phase 1: stop accepting new connections (HTTP 503),
                  // let existing ones finish naturally
hub.Shutdown(ctx) // phase 2: force-close anything still open
```

During drain, new upgrade attempts get rejected, but existing connections
keep working normally — they finish their in-flight request/response cycles
and disconnect on their own terms. Connections that go idle during this
window (no traffic for the drain timeout, default 30s) get proactively
closed with a proper `CloseGoingAway` (1001) frame rather than left hanging
until the hard deadline. Only what's left after that gets force-closed in
phase two. This is what makes a rolling deploy invisible to end users instead
of a visible blip every release.

## 6. Rate limiting protects the hub from its own clients

Connection-level backpressure protects against slow *readers*. It does
nothing against a client — malicious or just buggy — that opens connections
faster than you can process them, or floods messages at a rate no downstream
consumer can absorb. That needs limits enforced *before* the expensive part
of the pipeline runs:

- **Max total connections** — a hard ceiling on hub size, so a runaway client
  can't exhaust file descriptors for everyone else.
- **Max connections per user** — bounds the blast radius of one compromised
  or misbehaving account, and catches the "client library retries without
  backoff" bug before it becomes an incident.
- **Max rooms per client** — the room-membership equivalent; without it, one
  client subscribing to every room turns every room broadcast into a
  full broadcast.

The critical detail is *when* the user-ID limit gets checked. If a client can
exist unauthenticated and get a user ID assigned afterward, there's a window
where `MaxConnectionsPerUser` simply doesn't apply to it. Assigning the user
ID atomically as part of the upgrade — not as a second step after the
connection is already registered — closes that window.

## 7. You can't operate what you can't see

Every mechanism above — backpressure drops, drain progress, rate-limit
rejections — is invisible unless something is counting it. A production
WebSocket service needs, at minimum:

- A **metrics interface** your monitoring stack can plug into (connection
  count, messages sent/dropped, handler latency), kept as an interface
  rather than a hard Prometheus dependency, so libraries that don't want
  Prometheus in their dependency tree aren't forced to have it.
- **`/healthz` and `/readyz`** as distinct signals. Liveness answers "is the
  process alive"; readiness answers "should the load balancer route new
  connections here" — and during a drain, those two answers correctly
  diverge: alive, but not ready.
- **Lifecycle hooks** (`OnConnect`, `OnDisconnect`, `OnSendDropped`,
  `BeforeDisconnect`) as the seam where you attach logging, auditing, or
  cleanup, without reaching into the hub's internals to do it.

None of this is glamorous, and all of it is what turns a 3 a.m. page into a
five-minute diagnosis instead of a two-hour one.

## 8. One process is not enough — and pub/sub isn't optional

Eventually one machine's connection count or CPU is the ceiling, and you need
a second node. This is where a WebSocket server's statefulness bites: unlike
a stateless HTTP API, `SendToUser("alice", ...)` only works if *this* process
holds Alice's connection. If your load balancer routed her to node B and the
message originated on node A, a naive multi-node deployment silently drops
it.

The fix is a message bus between nodes — Redis pub/sub or NATS are the
common choices — behind a small adapter interface:

```go
type Adapter interface {
    Publish(ctx context.Context, msg AdapterMessage) error
    Subscribe(ctx context.Context, handler func(AdapterMessage)) error
    Close() error
}
```

Every broadcast and targeted send publishes to the bus in addition to
delivering locally, and every node subscribes and re-delivers to whichever of
*its own* local connections match the target. Two details make this correct
rather than merely functional:

- **Node-ID deduplication.** Without tagging each published message with its
  originating node, a broadcast fans out once locally and once again when the
  node receives its own message back off the bus.
- **Local delivery must never block on the adapter.** If Redis is slow or
  down, clients on a healthy node should keep working. The adapter publish
  path degrades independently from the local dispatch path — an adapter
  outage should never become a local outage.

If you also need cluster-wide counts (`GlobalClientCount`, `GlobalRoomCount`)
rather than per-node numbers, that's a separate periodic presence-gossip
mechanism, not something adapters give you for free — and it's worth keeping
optional, since not every deployment needs it and gossip has its own cost.

## What this adds up to

None of these eight problems is individually hard. What makes a WebSocket
server "production-grade" is that all eight have to be solved *together*, and
several of them interact — backpressure policy changes what a slow client
during drain looks like; rate limiting has to compose with the upgrade path's
atomicity; the adapter layer has to respect the same non-blocking guarantee
as local broadcast. Solve them one at a time in isolation and you'll find the
seams later, under load, at the worst time.

I built these ideas into an open-source Go package called
[**wshub**](https://github.com/KARTIKrocks/wshub) — rooms, backpressure with
configurable drop policies, two-phase drain/shutdown, connection and rate
limits, a pluggable metrics interface with an official Prometheus
subpackage, and Redis/NATS adapters for horizontal scaling, all benchmarked
and MIT-licensed. It's built on top of `gorilla/websocket`, not a replacement
for it — the protocol handling was never the hard part.

```text
go get github.com/KARTIKrocks/wshub
```

If you're currently maintaining a hand-rolled `Hub struct` with a
`map[*Client]bool` and a `broadcast chan []byte`, most of what's above is
probably already on your TODO list. The
[documentation](https://kartikrocks.github.io/wshub/) has the full API, and
the [README's benchmark section](https://github.com/KARTIKrocks/wshub#benchmarks)
has the methodology behind every number quoted here.
