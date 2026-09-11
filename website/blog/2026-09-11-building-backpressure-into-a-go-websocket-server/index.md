---
slug: building-backpressure-into-a-go-websocket-server
title: Building Backpressure into a Go WebSocket Server
authors: [kartik]
tags: [go, websocket, concurrency, reliability]
description: A bounded channel and a non-blocking send get you 80% of the way to correct backpressure. The other 20% is two specific races that only show up under concurrent load — and they're worth understanding before you hit them in production.
---

# Building Backpressure into a Go WebSocket Server

Every WebSocket server eventually gets a slow client — bad wifi, a suspended
mobile app, a browser tab that stopped running its event loop. The server
keeps producing messages for it regardless. What happens to those messages
is a design decision, and "block until the client catches up" is the wrong
one: block the wrong goroutine and you've turned one slow client into an
outage for every other connection on the process.

{/* truncate */}

This is article three in a series on production-grade WebSocket servers in
Go — [article one](/blog/production-grade-websocket-server-in-go) listed the
eight problems production traffic surfaces, [article two](/blog/handling-10000-websocket-connections-in-go)
went deep on connection-scaling arithmetic. This one is about the mechanism
that's easy to get *approximately* right and surprisingly easy to get
*exactly* wrong: dropping messages under load without dropping correctness
along with them. The code below is from [wshub](https://github.com/KARTIKrocks/wshub)'s
actual send path, not a simplified sketch — the races it guards against are
the reason the "obvious" implementation isn't the one that shipped.

## The 80% version, and why it's not enough

The standard advice for a slow-consumer problem is: don't write to the
socket directly, write to a bounded channel, and make the write
non-blocking.

```go
select {
case client.send <- data:
    // enqueued
default:
    // buffer full — drop it
}
```

This is correct as far as it goes, and it's most of what you need. A
`writePump` goroutine drains `client.send` and owns the actual socket
writes, so a full buffer never blocks the caller — broadcast, targeted send,
whatever code path got you here returns immediately either way. This is why
`SendToClient` costs ~105 ns with zero allocations at 100K clients in
wshub's benchmarks: the send path never waits on I/O, only on a channel
operation that's guaranteed not to block.

But two things are still unhandled, and both of them only show up under
concurrency — which is exactly why they're easy to ship without noticing.

## Race 1: the client can close between your check and your send

A `Client` has a `closed` flag, set when the connection tears down. The
obvious guard is to check it before sending:

```go
if client.closed {
    return false // don't bother sending
}
client.send <- data
```

This looks safe and isn't. Nothing prevents the client from disconnecting —
and its `writePump` from closing `client.send` — in the window between the
check and the send. That's a plain time-of-check-to-time-of-use race, and
when it loses, the result isn't a dropped message: it's a panic. Sending on
a closed channel is one of the handful of things Go will not let you
recover from gracefully by checking state first, because there's no atomic
"check and send" primitive for channels the way there is for, say,
`sync/atomic.CompareAndSwap`.

wshub's actual send path still does the `closed` check — as a fast-path
optimization, to skip the panic/recover machinery for the common case where
a client is already known to be gone — but it doesn't *trust* that check.
Every send is wrapped in a `recover`:

```go
defer func() {
    if r := recover(); r != nil {
        if isChanSendPanic(r) {
            ok = false // client.send was closed — the client is disconnecting
            return
        }
        panic(r) // re-panic for anything else
    }
}()
```

The `isChanSendPanic` check matters more than it looks. Go doesn't give you
a typed error for "send on closed channel" — it's a runtime panic carrying a
plain string, and that string is the only signal available:

```go
func isChanSendPanic(r any) bool {
    // ...
    return strings.Contains(msg, "send on closed channel")
}
```

The re-panic on anything else is the important part of this design, not an
afterthought. A `recover` that swallows *every* panic turns real bugs — a
nil pointer dereference two calls deep in a message handler, an index out
of range in application code — into silent, undebuggable message drops.
Matching the specific panic string and re-panicking on everything else means
this `recover` catches exactly the one race it exists to catch, and gets out
of the way of every other failure.

## Race 2: evicting under `DropOldest` isn't one operation either

`DropNewest` (discard the message that didn't fit) is race-free almost by
definition — there's nothing to coordinate, you just don't enqueue. `DropOldest`
(evict the head of the queue to make room for the new message, because only
the latest state matters) is a different story: it's two channel operations
— a receive to evict, a send to enqueue — and if two goroutines hit this
path for the same client at once, both can decide the buffer is full,
both start evicting, and you can end up dropping two messages to make room
for one.

wshub serializes this specific sequence with a per-client mutex, and only
for this path — the fast, common non-blocking send above never touches it:

```go
func (h *Hub) trySendDropOldest(client *Client, item sendItem) bool {
    client.sendMu.Lock()
    defer client.sendMu.Unlock()

    // Re-check: the buffer may have drained while we waited for the lock —
    // writePump could have pulled a message off it in the meantime.
    select {
    case client.send <- item:
        return true
    default:
    }

    // Evict the oldest message, then enqueue the new one. Retry twice: a
    // concurrent fast-path sender (one that hasn't hit this lock at all)
    // can still slide into the slot we just freed before our own enqueue
    // runs, so the first evict+enqueue attempt can lose the race and needs
    // a second try rather than giving up and dropping the new message too.
    for range 2 {
        select {
        case dropped := <-client.send:
            h.notifySendDropped(client, dropped.data)
        default:
        }
        select {
        case client.send <- item:
            return true
        default:
        }
    }
    return false
}
```

The mutex only has to serialize *this* path against *itself* — the
non-blocking fast path everywhere else still runs lock-free, so `DropNewest`
(the default) pays none of this cost, and `DropOldest` pays it only on the
buffer-full path, not on every send. That's the actual tradeoff `DropOldest`
buys you: strictly-correct "keep only the newest N" semantics, at the cost
of a mutex acquisition exactly when a client is already struggling to keep
up — which is the one time you can least afford a slow send path, and
precisely why the retry logic exists instead of a simpler "evict once, give
up if that races."

## Choosing a policy is a semantics question, not a performance one

Given the two policies work at different cost, it's tempting to treat
`DropNewest` as the "cheap" default and `DropOldest` as an opt-in for when
you need it. That's true, but it's not the deciding factor — the deciding
factor is what a dropped message *means* for your protocol:

- **`DropNewest`** is correct for append-only streams — chat messages, log
  lines, discrete events — where every message is independently meaningful
  and losing the newest of several queued ones is recoverable; the client
  still gets a consistent, if incomplete, history.
- **`DropOldest`** is correct for state-replacement streams — a live cursor
  position, a stock price, a presence indicator — where only the *current*
  value matters and delivering a stale one is actively wrong. Here, dropping
  the *new* message under `DropNewest` would mean showing the client
  outdated state for longer, which is worse than showing it fewer updates.

Get this backwards — `DropOldest` on a chat feed, say — and you get subtly
corrupted output (a client that sees "waves goodbye" without ever having
seen "says hello") instead of a clean gap, which is a harder bug to notice
in code review and a harder one to explain in an incident writeup.

## A drop is not allowed to be silent

Every drop, on either policy, runs through one function:

```go
func (h *Hub) notifySendDropped(client *Client, data []byte) {
    h.logger.Warn("Client send buffer full, message dropped", "clientID", client.ID)
    h.metrics.IncrementMessagesDropped()

    if h.hooks.OnSendDropped != nil {
        h.hooks.OnSendDropped(client, data)
    }
}
```

Three separate outlets, and each earns its place:

- **The metric** (`wshub_messages_dropped_total` in the Prometheus
  subpackage) is what you graph and alert on. A drop rate that's zero for
  weeks and then climbs is a real signal — a client of yours changed
  behavior, or you're underprovisioned for current traffic — and it's
  invisible without this.
- **The hook** (`OnSendDropped`) is the extension point for a policy
  decision the library can't make for you: disconnect a client after N
  consecutive drops, downgrade them to a lower-fidelity message format, or
  page someone if drops start hitting a specific high-value client. This is
  where `client.CloseWithCode` earns its keep, called from inside the hook.
- **The log line** is for incident diagnosis — which client, roughly when.

Worth knowing before you rely on it in production: that log line is not
rate-limited. It's a distinct code path from wshub's handshake-failure
logging, which *is* explicitly rate-limited — because a failed handshake is
something an unauthenticated client can trigger for free — while a message
drop only happens for a client the hub has already accepted and is actively
serving. That's a reasonable place to draw the line, but it also means a
genuinely pathological slow client under sustained broadcast load can
produce one `Warn` line per dropped message. If you're running at a scale
where that's a real concern, the fix isn't in the log line — it's sampling
or de-duplicating in your own `OnSendDropped` hook, which sees every drop
before you decide whether it's worth another log entry.

## What this buys you

None of this changes the big picture from article one: bound the buffer,
make the send non-blocking, pick a drop policy that matches your protocol's
semantics, and make every drop observable. What changes with the code above
is confidence that the mechanism is actually correct under concurrency, not
just correct in the case your tests happened to exercise — a
time-of-check-to-time-of-use race and a lost-update race are exactly the
kind of bugs that pass every test you write single-threaded and show up
three weeks into production, at 2 a.m., under real concurrent load.

```text
go get github.com/KARTIKrocks/wshub
```

The full `DropPolicy` and hook reference is in the
[docs](https://kartikrocks.github.io/wshub/docs/hub#drop-policy), and the
send-path benchmarks referenced above are reproducible with
`go test -bench=. -benchmem ./...` — methodology in the
[README](https://github.com/KARTIKrocks/wshub#benchmarks).
