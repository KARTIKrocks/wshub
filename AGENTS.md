# AGENTS.md

Guidance for AI coding agents working in the **wshub** repository.

## What this is

`wshub` is a production-ready, horizontally scalable WebSocket package for Go
(`github.com/KARTIKrocks/wshub`). It manages WebSocket connections with rooms,
broadcasting, multi-node clustering, middleware, hooks, backpressure control,
and observability. It is pure infrastructure — it ships **no business logic**.

The central type is `Hub` (`hub.go`), created with `NewHub` and configured via
functional options (`options.go`). See `doc.go` for the package-level overview.

## Module layout

This is a **multi-module** repository. Each of these has its own `go.mod` and
must be built/tested independently:

- `.` (root) — core package, no third-party runtime deps beyond the WebSocket lib.
- `prometheus/` — official Prometheus metrics subpackage.
- `adapter/redis/` — Redis Pub/Sub adapter for multi-node scaling.
- `adapter/nats/` — NATS core Pub/Sub adapter for multi-node scaling.

The core package ships **no adapter implementations** on purpose — adapters live
in submodules so consumers don't pull in unwanted dependencies. When touching the
`Adapter` interface (`adapter.go`), update both `adapter/redis` and `adapter/nats`.

### Key source files (root)

- `hub.go` — core Hub, dispatch loop, connection lifecycle, draining, shutdown.
- `client.go` — per-connection read/write pumps.
- `config.go` / `options.go` / `limits.go` — configuration & builder pattern.
- `adapter.go` — multi-node `Adapter` interface.
- `router.go` / `middleware.go` / `hooks.go` — message routing & extensibility.
- `presence.go` — cross-node presence gossip for global counts.
- `metrics.go` / `health.go` — observability and K8s probes.

## Build, test, and lint

**Always run `make all` after making changes** — it vets, lints, tests, and
builds **every module**, not just the root. This is the expected validation gate.

```bash
make all              # fmt + vet(+modules) + lint(+modules) + test(+modules) + build(+examples)
make test             # root module only: go test -race -count=1 ./...
make test-modules     # adapter/redis, adapter/nats, prometheus
make lint             # golangci-lint on the root module (make setup installs it if missing)
make lint-modules     # golangci-lint on the nested modules
make work             # create the gitignored go.work used by the module targets
make fix              # gofmt + goimports + golangci-lint --fix
make bench            # benchmarks with -benchmem
make fuzz             # fuzz targets (30s each)
make cover            # coverage report -> coverage.html
```

- Tests **must pass with the race detector** (`-race`). Concurrency correctness
  is a first-class requirement here.
- `make lint` / `make setup` require `golangci-lint` v2 and `goimports`
  (pinned versions in the `Makefile`).
- The root module's `./...` stops at nested `go.mod` boundaries, so the module
  targets above are what cover the adapters and the Prometheus collector. They
  resolve `wshub` through a gitignored `go.work` pointing at the working tree,
  which is how CI runs them too (via `go mod edit -replace`) — so a breaking
  change in the root module fails locally rather than after release.

## Conventions

- **Go 1.22+** for the core module (submodules pin newer versions in their `go.mod`).
- Follow standard Go style; `gofmt` + `goimports` are mandatory (`make fmt`).
- **All exported types and functions must have doc comments.**
- Keep the hot path zero-allocation — benchmarks in `benchmark_test.go` guard
  dispatch performance. Verify with `make bench` if you touch dispatch/send code.
- **Never `close(client.send)` in library code.** Producers (`Hub.trySendErr`,
  `Client.SendMessageWithContext`) send on that channel without holding any lock
  a closer could take, so closing it races with them. Shutdown is signalled
  through `client.done` instead — `CloseWithCode` sets `graceful` so `writePump`
  flushes the queue and sends a close frame; `handleUnregister` drains the
  buffer. Tests may close `send` on a bare `&Client{}` with no `writePump`
  running to exercise the recover guards.
- Keep test coverage high; add tests alongside new functionality.
- Update `README.md` / `doc.go` when the **public API** changes, and add a
  `CHANGELOG.md` entry for user-facing changes.

## Pull requests & commits

- Keep changes focused on a single concern; include tests.
- Ensure `make all` (or at least `make ci`) passes before opening a PR.

## Examples & load testing

- `examples/` — runnable examples (simple, chat, auth, metrics, multinode).
- `cmd/loadtest/` — real-WebSocket load generator: `make loadtest LOADTEST_ARGS="-scenario=fanout -clients=10000"`.

## Documentation website

The docs site is **not on `main`** — it lives on the separate **`website`** branch
under `wshub-website/` (React 19 + Vite 7 + TypeScript + Tailwind v4 + Shiki). It is
built and published to GitHub Pages (the `gh-pages` branch) with `npm run deploy`,
served under the `/wshub/` base path.

The site is **version-aware**: it fetches releases from the GitHub API and renders
docs for the selected version. Two things must be kept in sync when the library
changes:

- **New release** — bump `LATEST_VERSION` in `src/components/VersionProvider.tsx`.
- **Public API change** — update the matching topic file in `src/content/*.tsx`
  (one per subsystem: `hub`, `client`, `rooms`, `adapter`, `presence`, etc.). New
  features that only exist from a given version are gated in `src/App.tsx` via
  `minVersion('vX.Y.Z')`, so add the gate there when a doc section is version-specific.

When you change the **public API** on `main`, mirror the change on the `website`
branch so the published docs don't drift.
