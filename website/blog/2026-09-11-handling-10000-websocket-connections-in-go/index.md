---
slug: handling-10000-websocket-connections-in-go
title: How to Handle 10,000 WebSocket Connections in Go
authors: [kartik]
tags: [go, websocket, performance, systems]
description: 10,000 concurrent connections stopped being a hard problem for Go's runtime a decade ago. What's left is a small, checkable list of OS limits and memory arithmetic — here's the actual accounting.
---

# How to Handle 10,000 WebSocket Connections in Go

"Can it handle 10,000 connections?" gets asked about every realtime service,
usually as a stand-in for "is this production-grade." It's the wrong question
in one specific way: for a Go server, holding 10,000 idle-ish WebSocket
connections open was never the hard part. The C10K problem — the one that
gave the number its mythology — was about thread-per-connection servers
where each connection cost an OS thread and a context switch. Go's runtime
solved that structurally before you wrote a line of your handler. What's
left is arithmetic: how much memory each connection actually costs, and
which OS limits you'll hit before your own code does.

{/* truncate */}

This is article two in a series on what "production-grade" means for a Go
WebSocket server — [the first one](/blog/production-grade-websocket-server-in-go)
listed eight problems production traffic surfaces. This one goes deep on the
first number everyone asks about, using [wshub](https://github.com/KARTIKrocks/wshub)'s
own published load-test numbers as the concrete case.

## Why the C10K framing is stale

The original C10K problem (Dan Kegel, 1999) was about servers that spent one
OS thread per connection. Ten thousand threads meant ten thousand ~1–8 MB
stacks and a kernel scheduler context-switching between them — real
resource pressure, independent of what the handler actually did.

Go doesn't have that problem by construction. A goroutine's stack starts at
2 KB and grows on demand; the runtime's own scheduler multiplexes goroutines
onto a small, `GOMAXPROCS`-bound pool of OS threads. Critically, a goroutine
blocked on `conn.Read()` or `conn.Write()` doesn't block an OS thread at
all — the runtime's netpoller (backed by `epoll` on Linux, `kqueue` on
BSD/macOS) parks it and reuses the thread for other goroutines until the
socket is actually readable or writable. Ten thousand goroutines blocked on
network I/O cost ten thousand parked stacks, not ten thousand blocked
threads. This is what makes goroutine-per-connection — rather than an
event-loop-with-callbacks architecture — a reasonable default in Go, when
the same model was a liability in C or early Java.

So "can Go handle 10K connections" was already answered by the runtime.
The real questions are narrower:

1. How much memory does each connection actually cost, end to end?
2. What OS-level limits sit between your process and 10,000 open sockets?
3. At what point does something *other* than connection count become the
   bottleneck?

## The actual memory budget

wshub's connection handler is exactly two long-lived goroutines per client —
a `readPump` blocked in `conn.ReadMessage()`, and a `writePump` draining a
buffered `chan []byte` and owning all writes to the socket (`gorilla/websocket`
requires a single writer; concurrent writes from two goroutines is a data
race the library doesn't protect against). The HTTP handler goroutine that
performed the upgrade returns immediately after spawning those two — it
does not stick around, so it's not part of the steady-state cost.

Per connection, that's:

- Two goroutine stacks (2 KB initial, growing only if a call frame needs it —
  in practice these pumps stay shallow).
- Two `gorilla/websocket` I/O buffers, 1 KB each by default
  (`ReadBufferSize`/`WriteBufferSize` in wshub's `Config`).
- A buffered send channel, `SendChannelSize` slots deep (default 256) — note
  this holds `[]byte` *references*, not preallocated byte arrays, so it costs
  roughly 256 × 24 bytes of slice headers when full, not 256 KB.
- Bookkeeping: a map entry in the hub's client registry, connection metadata,
  and whatever callbacks are registered.

Measuring this in aggregate rather than adding up estimates is more honest,
so here's what wshub's end-to-end load test actually measures (real
`gorilla/websocket` dialer connections against a real `httptest.Server`,
memory sampled via `runtime.MemStats` and divided by connection count):

| Clients | Connect time | Rate          | Mem/conn |
| ------- | ------------ | ------------- | -------- |
| 1,000   | 59 ms        | 15,754 conn/s | 27.1 KB  |
| 5,000   | 162 ms       | 29,853 conn/s | 24.0 KB  |
| 10,000  | 263 ms       | 36,891 conn/s | 25.9 KB  |

Two things worth noticing. First, per-connection memory is flat across an
order of magnitude of scale (~25–27 KB) — there's no hidden O(n) cost
lurking in the client registry or the broadcast path that shows up only at
higher connection counts. Second, handshake *throughput* actually improves
with scale (15.7K → 36.9K conn/s), which is a load-test-harness artifact, not
a server property: connecting more clients amortizes fixed startup costs
(goroutine scheduling ramp-up, initial GC assist) over more connections. At
10,000 clients that's still only ~260 ms wall-clock to establish every
connection.

Multiply 25.9 KB by however many connections you're planning for and you
have a real memory floor: 100,000 idle connections is roughly 2.6 GB just
for connection-holding, before your application logic allocates anything.
That's the number to put in a capacity plan, not a guess.

## The OS limits, in the order you'll hit them

None of these are Go-specific, and all of them are checkable before you
find out about them from an outage.

**1. File descriptor limit (`ulimit -n`).** Every WebSocket connection is one
socket, and every socket is one file descriptor. Most Linux distributions
default the per-process soft limit to 1024 — far below 10,000. Check it with
`ulimit -n`, and raise it wherever the process actually starts:
`LimitNOFILE=65536` in a systemd unit, `--ulimit nofile=65536:65536` for
Docker, or `ulimit -n 65536` before exec in a shell-launched process. Budget
headroom above your target connection count for log files, outbound
connections to Redis/NATS if you're using an adapter, and health-check
sockets.

**2. Listen backlog (`net.core.somaxconn`).** This one only bites during a
*connection burst*, not at steady state. The kernel queues completed
TCP handshakes for `accept()` in a backlog whose size is capped by
`net.core.somaxconn` (historically 128 on older kernels, higher on modern
defaults); Go's `net.Listen` requests a backlog but the kernel silently caps
it at this value. If 5,000 clients try to connect within the same
handshake-timeout window — a mobile app coming back online after a network
blip, a game client reconnect storm — and your backlog is smaller than the
burst, the kernel starts dropping or refusing the excess before your accept
loop ever sees them. This is exactly why wshub's own load-test client caps
itself to 200 concurrent in-flight handshakes with a semaphore: not because
the server can't take more, but because an unbounded connect burst measures
the *test harness's* backlog behavior instead of the server's. In
production, size `somaxconn` to your worst realistic reconnect-storm burst,
not your steady-state connection count.

**3. Ephemeral ports — but check which side actually needs them.** This is
where a lot of write-ups get it backwards. A listening server doesn't
consume an ephemeral port per accepted connection — every inbound connection
shares the same server-side `(IP, port)` pair; what makes each connection
unique is the *client's* IP and port. The ephemeral port range
(`net.ipv4.ip_local_port_range` on Linux, roughly 28,000–60,000 ports by
default) only matters for the side *initiating* outbound connections: a
load-generator opening 10,000 client sockets from one machine, or your
server's own outbound connections to a Redis/NATS adapter. If you're only
accepting inbound WebSocket connections, this limit is irrelevant to your
connection ceiling — don't spend time tuning it based on advice written for
a different topology.

**4. `GOMAXPROCS` and the scheduler, not connection count.** wshub's load
test header prints `12 cores` on the machine these numbers came from — this
matters for *dispatch* work, not for holding connections open. A goroutine
parked on a socket read costs no CPU; a goroutine actively processing a
message does. The next section is where this actually shows up.

## Where 10,000 stops being "just a number"

Holding 10,000 idle connections open is close to free. *Doing something*
with all of them concurrently is a different problem, and it's where wshub's
own benchmarks show a real cliff — not in connection count, but in
sustained fanout:

| Clients | Throughput    | p50     | p95     | p99     |
| ------- | ------------- | ------- | ------- | ------- |
| 1,000   | 100,000 msg/s | 1.48 ms | 1.85 ms | 2.75 ms |
| 5,000   | 499,500 msg/s | 7.91 ms | 19.0 ms | 31.8 ms |
| 10,000  | 693,900 msg/s | 1.72 s  | 3.19 s  | 3.34 s  |

Throughput keeps climbing, but p50 latency jumps almost three orders of
magnitude between 5,000 and 10,000 clients under sustained broadcast load
(100 msg/s from a single broadcaster, 10-second window). The documented
explanation is scheduler pressure: every broadcast at this scale is pushing
into thousands of buffered channels, and *reading* off each of those
channels and writing to each socket is real, concurrently-scheduled CPU and
syscall work — competing for the same `GOMAXPROCS`-bound thread pool that
every other goroutine in the process shares, including the read side of the
load-test harness itself. The hub's own dispatch loop stays allocation-free
at this scale (the in-process broadcast micro-benchmarks confirm that part
in isolation); the cost is downstream of dispatch, in the fan of individual
socket writes.

The practical implication: "10,000 connections" and "10,000 connections all
receiving a broadcast every 10ms" are different capacity questions, and the
second one is the one that actually constrains a single node. wshub exposes
three knobs for this, in the order I'd reach for them:

- **`SendChannelSize`** — a deeper per-client buffer absorbs short bursts
  without invoking the drop policy, at the cost of more memory per
  connection and, if actually filled, more end-to-end latency for whatever's
  sitting in the queue.
- **`WithCoalesceWrites(true)`** — batches queued text messages into a
  single WebSocket frame (newline-separated) before they hit the socket,
  trading a small amount of client-side parsing for fewer syscalls per
  connection under high message rates.
- **Horizontal scaling** — past a real per-node ceiling, add nodes behind a
  Redis or NATS adapter rather than tuning a single process further. That's
  its own set of correctness problems (node-ID deduplication, not blocking
  local delivery on the bus) — enough for the next article in this series.

One knob that's *not* on this list: `WithParallelBroadcast`. It exists for
backward compatibility, but wshub's own load tests found it consistently
slower than the default serial broadcast in practice — the per-call
lock/defer overhead of parallel dispatch doesn't pay for itself against real
sockets, even though it looks like it should help on paper. Measure before
reaching for parallelism; the bottleneck here isn't the dispatch loop.

## The checklist

Getting a Go process to comfortably hold 10,000 WebSocket connections is not
a performance-engineering project. It's five checks:

1. **`ulimit -n`** raised above your target connection count plus headroom,
   at the layer that actually launches the process (systemd/Docker/shell).
2. **`net.core.somaxconn`** sized to your worst realistic reconnect burst,
   not your steady-state count.
3. **Memory budget** = connection count × ~26 KB (adjust for your own
   `SendChannelSize`/buffer configuration) — a real number for capacity
   planning, not a guess.
4. **Don't tune ephemeral ports on the server** unless it's also making
   10,000 outbound connections — that limit belongs to whoever dials, not
   whoever listens.
5. **Benchmark fanout, not just connect** — a server holding 10,000 idle
   connections and one broadcasting to all of them every 10ms are different
   capacity questions, and the second is where you'll actually find your
   ceiling.

Every default referenced here — buffer sizes, channel depth, coalescing,
drop policy — is a `wshub.Config` or `Hub` option, not something you have to
reimplement to test for yourself:

```text
go get github.com/KARTIKrocks/wshub
```

Reproduce the numbers in this post directly:

```bash
make loadtest LOADTEST_ARGS="-scenario connect -clients 10000"
make loadtest LOADTEST_ARGS="-scenario fanout -clients 10000"
```

Full configuration reference is in the
[docs](https://kartikrocks.github.io/wshub/docs/configuration), and the
complete benchmark methodology is in the
[README](https://github.com/KARTIKrocks/wshub#benchmarks).
