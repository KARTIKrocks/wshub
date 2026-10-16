---
slug: another-websocket-library-for-go
title: "Why I Built Another WebSocket Library for Go (and Why It Might Be the Last One You Need)"
authors: [kartik]
tags: [go, websocket, opensource, backend]
description: Go already has several good WebSocket libraries. None of them solve the problem that actually eats a week of engineering time — so here's an honest survey of the landscape, and where the gap actually is.
---

# Why I Built Another WebSocket Library for Go (and Why It Might Be the Last One You Need)

"Another WebSocket library" is a fair thing to be skeptical of. Go already
has several good ones, some of them excellent at exactly what they set out to
do. So before explaining why [wshub](https://github.com/KARTIKrocks/wshub)
exists, the honest version of this post has to start somewhere else: what's
actually already out there, and what each one leaves for you to build
yourself.

{/* truncate */}

## The landscape, as it actually is

**[`gorilla/websocket`](https://github.com/gorilla/websocket)** is the
long-standing default, and it's what wshub is built on top of. It gives you
one thing well: a `Conn` with `ReadMessage`/`WriteMessage`, ping/pong,
close-frame handling, and `permessage-deflate` support — a correct
implementation of RFC 6455, nothing more. Everything past "I have a frame" —
registries, rooms, fanout, backpressure — is yours to write.

**[`coder/websocket`](https://github.com/coder/websocket)** (the maintained
continuation of `nhooyr.io/websocket`) is the modern take on the same layer:
zero dependencies, first-class `context.Context` support, a smaller surface
area than gorilla's. It's a genuinely nicer API to hold a single connection
with. It makes the same choice gorilla does, though — transport only. No
rooms, no broadcast primitive, nothing about what happens when you have ten
thousand of these open at once.

**[`gobwas/ws`](https://github.com/gobwas/ws)** goes a level lower than
either: raw frame headers, your own buffer reuse, your own masking. It exists
for people who want to hand-roll the protocol for performance reasons gorilla
and coder's abstractions get in the way of. Correct and fast, and further
from "batteries included" than any library on this list.

**[`melody`](https://github.com/olahol/melody)** is the closest thing that
already exists to what wshub is going for — a thin convenience layer over
gorilla that adds broadcast-to-all, broadcast-to-filtered, per-session
key/value storage, and automatic ping/pong timeout handling. If what you need
is "send this to everyone, and don't leak dead connections," melody solves
it in a dozen lines and you should probably use it. What it doesn't have:
rooms as a first-class concept, a backpressure policy when a slow client's
buffer fills, or any story for running more than one node.

**[`centrifuge`](https://github.com/centrifugal/centrifuge)** (and its
companion server, **Centrifugo**) sits at the other end entirely. It's a
real-time messaging platform — channels, presence, history/recovery on
reconnect, JWT-based auth — either embedded as a library or run as its own
clustered server that your services talk to over a client SDK. It's the
right answer if you're building the realtime equivalent of a message bus and
want most of the plumbing decided for you. It's also a bigger adoption
decision than a single WebSocket endpoint usually calls for: a new protocol
surface between your services and your clients, and in the server form, a
whole additional system to run and operate.

None of this is a knock on any of them. They're each solving the problem
they set out to solve. The gap is what's left standing between "a raw
transport library" and "a hosted real-time platform" — an **embeddable hub
layer**: something you `go get` into your own service, not a server you
stand up next to it, that gives you rooms, backpressure, graceful shutdown,
and multi-node scaling as library code you control.

## What that layer actually has to do

This isn't abstract. Across the rest of this series, the specific problems
that show up the moment a `gorilla/websocket` demo meets real traffic — and
that neither a transport library nor a full platform server quite hands you
in the shape a single Go service wants — were:

- **A registry and rooms.** Someone has to track which `*Client` values are
  live and which room each one belongs to, safely under concurrent
  connect/disconnect. [Article 1](/blog/production-grade-websocket-server-in-go)
  walks through why a plain map behind a mutex isn't enough once broadcast
  and unregister can run at the same time.
- **Backpressure with an actual policy, not a crash.** One slow client
  shouldn't be able to block a broadcast to everyone else, and an unbounded
  queue for that client is just a slower way to run out of memory. wshub's
  answer — bounded per-client channels plus a configurable drop policy
  (`DropOldest`, `DropNewest`, or none) with a notification hook — is the
  subject of [article 3](/blog/building-backpressure-into-a-go-websocket-server).
- **A graceful shutdown that actually waits.** "Close every connection" is
  one line. Draining in-flight sends, handling a client that's open but
  silent, and doing it within a deadline instead of hanging a rolling
  deploy needs a small state machine — covered in
  [article 5](/blog/graceful-websocket-shutdown-in-go).
- **More than one node, correctly.** The moment you run two replicas behind
  a load balancer, "broadcast to everyone" has to cross a process boundary
  without double-delivering to clients on the node that originated the
  message. [Article 4](/blog/scaling-websockets-across-multiple-go-servers-with-redis)
  covers the adapter pattern wshub uses for this (Redis and NATS, pluggable,
  with node-ID dedup built in) — the part that a single-process library like
  melody has no story for at all, and that centrifuge solves by making you
  adopt its whole server instead.
- **Honest numbers for the question everyone asks first.**
  [Article 2](/blog/handling-10000-websocket-connections-in-go) measures
  what 10,000 open connections and a sustained broadcast actually cost, in
  real memory and real latency, instead of leaving you to extrapolate from a
  microbenchmark.

None of these are hard problems individually. They're the kind of thing a
competent team writes correctly on the third try, after shipping the first
two versions to production and finding out what they missed. wshub's
pitch is narrow: do that work once, benchmark it, and keep it zero-allocation
on the hot path (`SendToClient` is 105 ns/op with zero allocations at 100K
registered clients — reproducible with `go test -bench=. -benchmem ./...`),
so the team doesn't have to re-derive it.

## Is it "the last one you need"?

Honestly — depends on what you're building. If you want a real-time
platform with presence, message history, and reconnect recovery as a
managed concern, and you're fine adopting a server and a client SDK to get
it, centrifuge/Centrifugo is a more complete answer than wshub is trying to
be. If all you need is "broadcast this to everyone on one node," melody is
lighter weight and you don't need wshub's configuration surface at all.

Where wshub earns the "last one" framing is the specific, common middle
case: a single Go service that owns its own WebSocket endpoint, needs rooms
and backpressure and graceful shutdown to be correct from day one, and will
eventually need more than one node — without wanting to stand up a separate
real-time server to get there. That's the gap none of the libraries above
fill as a library, and it's the one wshub was built to close.

```bash
go get github.com/KARTIKrocks/wshub
```

The [room-based chat example](https://github.com/KARTIKrocks/wshub/tree/main/examples/chat)
is the fastest way to see rooms and broadcasting working end to end; the
full capability list is in the
[README](https://github.com/KARTIKrocks/wshub#why-wshub), and the rest of
this series goes deep on each piece above.
