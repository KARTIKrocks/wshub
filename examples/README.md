# Examples

Runnable programs demonstrating wshub. Each is a self-contained `main.go` —
`cd` into its directory (or `go run ./examples/<name>` from the repo root)
and open the printed URL in a browser.

Every server defaults to port `8080`; set `PORT` to run more than one at
once (e.g. `PORT=8081 go run ./examples/chat`).

| Example | Demonstrates |
| ---------------------------- | -------------------------------------------------------------------------- |
| [`simple`](simple) | The minimum viable server: broadcast every message back to all clients |
| [`chat`](chat) | Rooms, targeted broadcasting, lifecycle hooks, middleware, event routing |
| [`auth`](auth) | Authenticating connections with `BeforeConnect` / `AfterConnect` |
| [`metrics`](metrics) | Implementing `MetricsCollector` and exposing a `/metrics` endpoint |
| [`multinode`](multinode) | Horizontal scaling across two hubs via the Redis adapter |

Start with `simple`, then `chat` for the room/broadcast API surface. `auth`
and `metrics` are focused, single-concept examples.

## Running `multinode`

`multinode` is the one exception to everything above. It is its own Go module
(it pulls in `go-redis`), it needs a running Redis, and it ignores `PORT` —
it starts two hubs on fixed ports so you can watch a message cross between
them:

```bash
# 1. Start Redis (any local instance works; Docker is just one option)
docker run --rm -p 6379:6379 redis:7-alpine

# 2. Run both nodes
go run ./examples/multinode
```

Then open both nodes and send a message from either one — it arrives on both:

- Node A → <http://localhost:8081>
- Node B → <http://localhost:8082>

Each node also exposes `/stats` with its local and cluster-wide client counts.
Set `REDIS_ADDR` if Redis is not on `localhost:6379`.

For the full API reference, see the
[Documentation](https://kartikrocks.github.io/wshub/) and
[pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub).
