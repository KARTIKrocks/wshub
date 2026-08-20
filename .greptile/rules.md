# Style and pattern rationale

Context for the scoped rules in `config.json`. This file is freeform prose read
alongside the diff; `config.json` is what actually gates comment scope and
severity.

## Lock ordering

`Hub` documents its lock order as `mu → roomsMu → Room.mu → Client.mu →
userIndexMu`. A change that acquires these out of order, or holds one across a
blocking channel send or network write, is a deadlock risk even if nothing
currently exercises the interleaving.

## Wire formats are shared contracts

`AdapterMessage` (`adapter.go`) and `nodePresence` (`presence.go`) are decoded
by every other node in the cluster during a rolling deploy. A struct tag rename
or a field removal is a wire-compatibility break between old and new nodes, not
just a Go API change — treat it with the same weight as a public API break.

## `encoding/json/v2`

`message.go`, `presence.go`, `adapter/redis/redis.go`, and `adapter/nats/nats.go`
import `encoding/json/v2`, not `encoding/json`. This is deliberate — v2's
defaults are stricter (case-sensitive object-member matching, duplicate names
rejected, invalid UTF-8 rejected) and that's the whole reason for using it on
`Message.JSON`, which decodes attacker-controlled client input. Do not suggest
reverting to `encoding/json` v1 to "fix" a case-insensitive match or a lenient
UTF-8 read; that would undo the hardening. `AdapterMessage` and `nodePresence`
are both producer and consumer of their own tags, so case sensitivity is not a
concern there.

## Zero-allocation hot path

`benchmark_test.go` pins expected allocation counts for broadcast, targeted
send, and lookup paths. A regression there is a correctness issue for this
library, not a style nit — flag a new allocation on a path the benchmarks cover
even if the change is otherwise correct.

## Multi-module boundaries

This is five Go modules (root, `adapter/redis`, `adapter/nats`, `prometheus`,
`examples/multinode`), each with its own `go.mod`. A change to `adapter.go`'s
`Adapter` interface needs a matching change in both adapter submodules in the
same PR — the root module's own build won't catch the mismatch.
