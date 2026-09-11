---
slug: scaling-websockets-across-multiple-go-servers-with-redis
title: Scaling WebSockets Across Multiple Go Servers with Redis
authors: [kartik]
tags: [go, websocket, redis, distributed-systems]
description: A stateless HTTP API scales by adding replicas behind a load balancer. A WebSocket server doesn't, because the connection itself is the state — and the fix isn't "add Redis," it's a specific set of correctness rules for using it.
---

# Scaling WebSockets Across Multiple Go Servers with Redis

A stateless HTTP handler scales the boring way: put a load balancer in
front, add replicas, done — any instance can answer any request because no
instance holds anything the others need. A WebSocket server breaks that
model by definition. The connection *is* the state. If Alice is connected to
node B and your billing webhook fires on node A and calls
`hub.SendToUser("alice", ...)`, node A's hub has no idea Alice exists. The
message doesn't error, doesn't queue, doesn't retry — it just isn't
delivered, because from node A's point of view there's no client called
Alice.

{/* truncate */}

This is article four in a series on production-grade WebSocket servers in
Go — see [article one](/blog/production-grade-websocket-server-in-go) for
the full list of problems, and [article two](/blog/handling-10000-websocket-connections-in-go)
and [article three](/blog/building-backpressure-into-a-go-websocket-server)
for connection-scaling and backpressure. This one is about the specific
correctness rules a message bus has to satisfy before "just add Redis" is
actually a fix rather than a new set of bugs — using [wshub](https://github.com/KARTIKrocks/wshub)'s
own Redis adapter as the worked example.

## The interface has to be this small

Scaling out means every broadcast, every targeted send, needs a second path
alongside local delivery: publish it somewhere every other node is
listening. The interface for that "somewhere" is deliberately three methods:

```go
type Adapter interface {
    Publish(ctx context.Context, msg AdapterMessage) error
    Subscribe(ctx context.Context, handler func(AdapterMessage)) error
    Close() error
}
```

Small enough that Redis Pub/Sub, NATS, or a hand-rolled bus are all
one afternoon's implementation — the interface makes no assumption about
delivery guarantees, ordering, or persistence beyond "fire this at every
other node." That minimalism is deliberate: the correctness rules below live
in *how the hub uses* this interface, not in the interface itself, so any
transport that satisfies these three methods inherits them for free.

## Rule 1: local delivery never waits on the bus

Every broadcast method does two things, in this order:

```go
func (h *Hub) Broadcast(data []byte) {
    h.broadcast(sendItem{msgType: websocket.TextMessage, data: data})
    h.publishToAdapter(AdapterMessage{
        Type:    AdapterBroadcast,
        MsgType: websocket.TextMessage,
        Data:    data,
    })
}
```

Local delivery happens first, synchronously, and unconditionally.
Publishing to the adapter happens second, and a failure there is logged and
counted — never propagated back to break local delivery:

```go
func (h *Hub) publishToAdapter(msg AdapterMessage) {
    if h.adapter == nil {
        return
    }
    msg.NodeID = h.nodeID
    if err := h.adapter.Publish(h.ctx, msg); err != nil {
        h.logger.Error("adapter publish failed", "error", err, "type", msg.Type)
        h.metrics.IncrementErrors("adapter_publish")
    }
}
```

This ordering is the whole point. If Redis is down, degraded, or just slow,
every node's *local* clients keep working normally — they simply stop
receiving updates that originated on other nodes, which is a partial,
recoverable degradation instead of every WebSocket connection in the fleet
going down because the message bus had a bad five minutes. A message bus
that can take down local delivery when it fails isn't actually decoupled
from it.

## Rule 2: every node has to ignore its own echo

Here's the bug you'd write without thinking about it twice: Node A
broadcasts. Node A delivers locally *and* publishes to Redis. Every node
subscribed to that Redis channel — including Node A itself — receives the
message back. Without a check, Node A now delivers the same broadcast to
its own clients twice.

The fix is a single equality check on receive:

```go
func (h *Hub) handleAdapterMessage(msg AdapterMessage) {
    // Ignore messages originating from this node.
    if msg.NodeID == h.nodeID {
        return
    }
    // ... dispatch locally, and never re-publish.
}
```

`msg.NodeID` is stamped by the *publishing* node in `publishToAdapter`
above, not read back from the transport — Redis Pub/Sub doesn't tell you who
published a message, so the dedup key has to travel inside the payload. The
second half of that comment — "never re-publish" — matters just as much as
the dedup check: `handleAdapterMessage` calls the same local-delivery
functions the direct API does (`broadcast`, `sendToUserLocal`, and so on),
never `publishToAdapter`. Skip that distinction and a two-node cluster
turns into an infinite relay: A publishes, B receives and re-publishes, A
receives B's re-publish and re-publishes again, forever.

## Rule 3: closing the subscription correctly is harder than opening it

The Redis adapter's `Subscribe` spawns two goroutines, not one, and the
reason is specific to how `go-redis`'s `PubSub` type manages its own
lifetime:

```go
// Closing the PubSub is what closes the channel the receive loop ranges
// over, so it has to happen from outside that loop — a `defer sub.Close()`
// on the receive goroutine can only run once the loop has already exited,
// which never happens. go-redis does not tie the PubSub's lifetime to the
// context passed to Subscribe, so this watcher is also what makes context
// cancellation stop delivery.
go func() {
    defer a.wg.Done()
    <-ctx.Done()
    _ = sub.Close()
}()

go func() {
    defer a.wg.Done()
    ch := sub.Channel()
    for msg := range ch {
        // ...deserialize and dispatch
    }
}()
```

The receive loop's exit condition is "the channel closes," and the channel
only closes when something calls `sub.Close()` — so a naive single-goroutine
implementation that tries `defer sub.Close()` around the `for range` loop
deadlocks forever, because the deferred call can't run until the loop it's
supposed to unblock has already exited. A second, dedicated goroutine whose
only job is watching the context and closing the subscription from outside
the loop is what actually makes cancellation work.

There's a second, quieter race in the same function, in how the two
goroutines get counted:

```go
a.mu.Lock()
// ...
a.wg.Add(2)
a.mu.Unlock()

// ... (goroutines started here, after the lock is released)
```

`wg.Add(2)` happens *before* the lock is released and before either
goroutine actually starts — not after. Go's own `sync.WaitGroup`
documentation calls this out directly: calls to `Add` that increment the
counter from zero must happen before a `Wait` that could observe it. If
`Add` happened after starting the goroutines instead, `Close` could call
`wg.Wait()` in the gap between the goroutines being scheduled and the
`Add` call actually running, see a zero counter, and return immediately —
reporting the adapter closed while both goroutines are still mid-startup.
Reserving both slots synchronously, before releasing the lock, closes that
window entirely.

## What "scaling" actually looks like end to end

Put together, two `wshub` processes sharing one Redis instance behave like
this: a client connects to whichever node its load balancer routes it to;
`WithAdapter(redisAdapter)` and a stable `WithNodeID` are the only
configuration difference from a single-node setup; every broadcast and
targeted send reaches every connected client on both nodes, with each
message delivered exactly once per client no matter which node it landed
on. wshub ships this as a runnable example, not just a diagram —
[`examples/multinode`](https://github.com/KARTIKrocks/wshub/tree/main/examples/multinode)
starts two nodes against one Redis container and exposes `/stats` on each,
printing both `local clients` (`hub.ClientCount()`) and `global clients`
(`hub.GlobalClientCount()`) side by side, so the split is visible instead of
asserted.

That `GlobalClientCount` number comes from a separate mechanism —
presence gossip, opt-in via `WithPresence(interval)` — not from the adapter
itself. Every tick, each node publishes its own local client and room
counts as an `AdapterPresence` message; every other node caches the latest
report per node ID and evicts it if the reporting node goes quiet for three
missed ticks (`presenceTTL` defaults to `3 × interval` — `WithPresence(5 *
time.Second)` means a node that stops reporting for 15 seconds silently
drops out of everyone else's global count). This is intentionally decoupled
from message delivery: you can run multi-node broadcast without presence at
all, and add cluster-wide counts later without touching how messages move.

## Try it

```bash
go get github.com/KARTIKrocks/wshub/adapter/redis
```

```go
rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
adapter := redis.New(rdb)

hub := wshub.NewHub(
    wshub.WithAdapter(adapter),
    wshub.WithNodeID("node-a"),
    wshub.WithPresence(5*time.Second),
)
go hub.Run()
```

Run the two-node demo directly:

```bash
cd examples/multinode && go run .
```

The [Adapters guide](https://kartikrocks.github.io/wshub/docs/adapters)
covers the full interface and the NATS adapter for anyone not already
running Redis; the [Presence guide](https://kartikrocks.github.io/wshub/docs/presence)
covers the gossip mechanism in more depth. Next in this series: what it
takes to take one of these nodes *out* of the cluster without dropping a
single in-flight connection.
