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
and `metrics` are focused, single-concept examples. `multinode` is the odd
one out — it's its own Go module (it pulls in `go-redis`) and needs a local
Redis instance; see its header comment for exact setup.

For the full API reference, see the
[Documentation](https://kartikrocks.github.io/wshub/) and
[pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub).
