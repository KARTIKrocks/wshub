---
slug: graceful-websocket-shutdown-in-go
title: Graceful WebSocket Shutdown in Go
authors: [kartik]
tags: [go, websocket, concurrency, reliability]
description: "\"Wait for every connection to close on its own\" sounds graceful and isn't — a client that's connected but silent can block it forever. The actual implementation needs a state machine, an idle reaper, and a shutdown path that unblocks it."
---

# Graceful WebSocket Shutdown in Go

[Article one](/blog/production-grade-websocket-server-in-go) in this series
showed the two-phase pattern — `Drain` then `Shutdown` — as the answer to
"what happens to 8,000 open connections when a deploy kills the pod."
This one is about why it needs to be two phases with real machinery behind
each, not a `sync.WaitGroup.Wait()` and a signal handler. The naive version
— stop accepting connections, wait for every existing one to close on its
own — sounds graceful and has a hole in it big enough to hang your rollout:
a client that's connected but silent, sending and receiving nothing, never
triggers a natural disconnect. Wait for it to leave on its own and you're
not draining gracefully, you're waiting for Kubernetes' own termination
grace period to expire and `SIGKILL` you anyway — which is exactly the
outcome graceful shutdown exists to avoid.

{/* truncate */}

This is article five in a series on production-grade WebSocket servers in
Go — see [article one](/blog/production-grade-websocket-server-in-go) for
the full list of problems this series covers, and
[articles two](/blog/handling-10000-websocket-connections-in-go),
[three](/blog/building-backpressure-into-a-go-websocket-server), and
[four](/blog/scaling-websockets-across-multiple-go-servers-with-redis) for
connection scaling, backpressure, and multi-node delivery. This one walks
through [wshub](https://github.com/KARTIKrocks/wshub)'s actual `Drain`/
`Shutdown` implementation and the specific races each piece of it defends
against.

## A state machine, not a bool

A hub is in exactly one of three states — `StateRunning`,
`StateDraining`, `StateStopped` — stored as a single atomic `int32` rather
than guarded by a mutex. That choice matters on the read side: `Health()`
and `Ready()` are meant to answer instantly under real concurrent load
(a Kubernetes probe hitting `/readyz` every few seconds shouldn't contend
with connection-registration locks), so every state read is a lock-free
atomic load, not a mutex acquisition.

The transition into draining uses a compare-and-swap, and the reason is
concurrency, not style:

```go
if !h.state.CompareAndSwap(int32(StateRunning), int32(StateDraining)) {
    current := h.State()
    if current == StateStopped {
        return nil // already shut down
    }
    // current == StateDraining: fall through to wait on the same signal.
} else {
    // We won the CAS — we are the drain initiator.
    // ...start the idle reaper, etc.
}
```

`Drain` is safe to call more than once, including concurrently, because
exactly one caller wins the CAS and becomes "the initiator" — the one that
logs the start, checks for the zero-clients fast path, and spawns the idle
reaper. Every other caller, including one that arrives after draining has
already started, just falls through to waiting on the same completion
signal. Without the CAS, two goroutines calling `Drain` at the same time —
plausible if both an HTTP `preStop` hook and a signal handler try to
initiate shutdown — would both start their own idle reaper, wastefully and
redundantly walking the same client list on separate tickers.

## One completion signal, closed exactly once

That completion signal is a channel, `drainDone`, and it has two independent
writers: `Drain` itself closes it when every client has disconnected, and
`Shutdown` closes it to force-unblock any `Drain` call still waiting — for
instance if the drain timeout is about to expire and something needs to
force the issue immediately. Closing a channel that's already closed
panics, so both call sites go through the same `sync.Once`:

```go
h.drainOnce.Do(func() { close(h.drainDone) })
```

This is a small detail with an easy failure mode if you write it yourself
without a guard: exactly one of "drain finished naturally" or "shutdown
forced it closed" will happen first in any given run, and which one wins is
a race by construction — the whole point is that either path can trigger
completion. Coordinating that through `sync.Once` rather than a manual
"only close if not already closed" check with a boolean flag is what makes
it correct under the race instead of merely correct in whichever order you
happened to test.

## The idle reaper: what "graceful" actually requires

Here's the mechanism that turns "wait for clients to disconnect" from a
hope into a bound. During drain, a background goroutine periodically checks
every client's outbound send buffer:

```go
for _, client := range snap.slice {
    if len(client.send) == 0 {
        if _, tracked := firstIdleAt[client]; !tracked {
            firstIdleAt[client] = now
        } else if now.Sub(firstIdleAt[client]) >= h.drainTimeout {
            _ = client.CloseWithCode(websocket.CloseGoingAway, "server draining")
            delete(firstIdleAt, client)
        }
    } else {
        delete(firstIdleAt, client) // has pending work — not idle
    }
}
```

Precision matters here: this measures the *send* buffer being continuously
empty — "the server has had nothing left to tell this client for the full
drain timeout" — not literally whether the client is responsive. That's a
deliberate, narrower signal than "is this client still alive," and it's the
right one for this job: during a drain you're not trying to detect a broken
client, you're trying to bound how long you wait for ordinary, well-behaved
connections that simply have no current reason to disconnect on their own.
A chat client sitting on an open tab is not going to close its own socket
just because your server would like to restart — proactively closing it
with `CloseGoingAway` (1001), a real WebSocket close code, is what lets it
distinguish "the server said goodbye" from a dropped connection and
reconnect cleanly.

The check interval is derived from the timeout itself, floored so short
timeouts (tests, aggressive rollout policies) stay responsive:

```go
interval := max(h.drainTimeout/4, 100*time.Millisecond)
```

With the default 30-second drain timeout that's a 7.5-second tick, and the
worst case costs two of them, not one: a connection that goes idle right
after a tick can wait up to a full interval before `firstIdleAt` even gets
recorded, and the close itself only fires on a later tick too — so the real
bound on top of `drainTimeout` is up to `2 × interval` (~15s here), not
just one. That's a concrete, quantifiable bound you can reason about when
setting your own `preStop`
grace period, not an unspecified "eventually."

## Shutdown: the part that actually forces connections closed

`Drain` alone never force-closes an active connection — only idle ones past
the timeout. Whatever's left when the context passed to `Drain` expires is
`Shutdown`'s job, and it doesn't loop over clients calling `Close()`
individually. It cancels the hub's shared context:

```go
h.state.Store(int32(StateStopped))
h.drainOnce.Do(func() { close(h.drainDone) }) // unblock any pending Drain
h.cancel()
```

That single `cancel()` is what tears down every remaining connection, and
it works the same way [article four](/blog/scaling-websockets-across-multiple-go-servers-with-redis)'s
Redis adapter closes its subscription: a small watcher goroutine per client
does nothing but wait on `ctx.Done()` and then call `closeConn()`, which is
what actually unblocks the `readPump`'s blocking `ReadMessage()` call. It's
the same shape — a goroutine whose only job is to close something from
*outside* the blocking loop that's holding it — showing up twice in the
same codebase because it's the correct answer to the same underlying
problem: you can't make a blocking call return early from inside itself, so
something else has to close the resource it's blocked on.

One ordering detail in `Shutdown` is there specifically because of that
same adapter mechanism:

```go
// Close the adapter before waiting on goroutines — the subscriber
// goroutine may block on a channel read that only unblocks on close.
if h.adapter != nil {
    if err := h.adapter.Close(); err != nil {
        h.logger.Error("adapter close failed", "error", err)
    }
}
```

If `Shutdown` waited on its own `WaitGroup` first and closed the adapter
after, it could deadlock against exactly the pattern described in article
four: the adapter's receive goroutine is parked on `sub.Channel()`, which
only unblocks when something calls `sub.Close()` — and that call lives
inside the adapter's own `Close()`, not inside the hub. Closing the adapter
first guarantees that call happens before anything waits for it to finish.

Everything after that is bookkeeping in the right order — the worker pool
(if `WithParallelBroadcast` was configured) drains its remaining queued
work before its own workers exit, then `Shutdown` waits on the hub's
`WaitGroup` bounded by the passed context, returning `nil` on clean exit or
the context's error if the deadline hits first — the same
"select on completion vs. context" shape as `Drain`.

## Putting it back together

The two-phase call from article one now has a fuller explanation behind it:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

hub.Drain(ctx)    // stop new connections; idle ones get CloseGoingAway
                  // after drainTimeout; active ones finish on their own
hub.Shutdown(ctx) // cancel the shared context — force-closes whatever's left,
                  // closing the adapter first so its own goroutines can exit
```

Skipping `Drain` and calling `Shutdown` alone is a valid, different choice —
every connection gets force-closed immediately via context cancellation,
with no `CloseGoingAway` grace period. That's the right call for a genuine
emergency stop; it's the wrong one for a routine rolling deploy, which is
why the two are separate methods instead of one with a flag.

```text
go get github.com/KARTIKrocks/wshub
```

Full reference for `Drain`, `Shutdown`, and `WithDrainTimeout` is in the
[Hub docs](https://kartikrocks.github.io/wshub/docs/hub#graceful-drain).
That closes out this five-part series — from the eight problems a
production WebSocket server has to solve, through connection scaling,
backpressure, multi-node delivery, and now shutdown. The
[README](https://github.com/KARTIKrocks/wshub) has the full API surface and
benchmark methodology for anything not covered along the way.
