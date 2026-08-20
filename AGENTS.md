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
- `examples/multinode/` — its own module because it depends on `adapter/redis`.
  It ships no tests; CI builds it to catch API drift.

The submodules are **versioned independently** of the core package (the adapters
are on their own `v0.2.x` line) and each pins a published `wshub` version in its
`go.mod`. See the workspace note under *Build, test, and lint* for how that pin
is replaced with the working tree.

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
make vuln             # govulncheck on the root module (needs network)
make vuln-modules     # govulncheck on the nested modules
make work             # create the gitignored go.work used by the module targets
make tidy             # go mod tidy on the root module
make tidy-modules     # go mod tidy on the nested modules (run after bumping their wshub pin)
make tidy-check       # fail if any go.mod/go.sum is untidy, leaving no diff behind
make fix              # gofmt + goimports + golangci-lint --fix
make bench            # benchmarks with -benchmem
make fuzz             # fuzz targets (30s each)
make cover            # coverage report -> coverage.html
```

- Tests **must pass with the race detector** (`-race`). Concurrency correctness
  is a first-class requirement here.
- `make lint` / `make setup` require `golangci-lint` v2 and `goimports`
  (pinned versions in the `Makefile`). The ruleset in `.golangci.yml` is clean
  across every module — `gosec`, `errorlint`, `bodyclose`, `noctx`,
  `contextcheck` and `perfsprint` are all enabled on library code, and relaxed
  only for `_test.go`, `examples/` and `cmd/`.
- The root module's `./...` stops at nested `go.mod` boundaries, so the module
  targets above are what cover the adapters and the Prometheus collector. They
  resolve `wshub` through a gitignored `go.work` pointing at the working tree,
  which is how CI runs them too (via `go mod edit -replace`) — so a breaking
  change in the root module fails locally rather than after release.

## Conventions

- **Go 1.27+** for every module — the core package and all submodules declare the
  same floor.
- Follow standard Go style; `gofmt` + `goimports` are mandatory (`make fmt`).
- **New exported types and functions must have doc comments** starting with the
  identifier name — they are the pkg.go.dev reference. This is not yet enforced
  by lint (`revive`'s `exported` rule is disabled in `.golangci.yml`) because
  some existing methods in `metrics.go`/`logger.go` predate the rule; do not add
  to the backlog.
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
- **One branch per change.** Never add follow-up commits to a branch that has
  already been merged — branch again off updated `main`.
- **Branch names are `type/kebab-slug`**, using the same type as the commit:
  `fix/close-with-code-send-channel-race`, `ci/test-submodules`,
  `docs/docusaurus-website`.
- **Commits follow [Conventional Commits](https://www.conventionalcommits.org/)**:
  `type(optional-scope): imperative summary`, no trailing period. Types are
  `feat`, `fix`, `docs`, `test`, `ci`, `build`, `chore`, `perf`, `refactor`,
  `revert`. Security fixes are `fix(security):`; dependency bumps are
  `chore(deps):` (`ci(deps):` for Actions), which is what Dependabot is
  configured to emit.
- **PRs are squash-merged**, so the PR title becomes the commit on `main` and
  must itself be a valid Conventional Commit. Intermediate commits on the branch
  are working state and do not need to be tidy — they will be collapsed.
- Never push directly to `main`; every change goes through a PR, including
  single-line ones.
- CI (`.github/workflows/ci.yml`) tests the root module and each submodule at
  two self-tracking legs: `go-version-file` (whatever that module's own `go.mod`
  declares as the floor) and `stable` (whatever Go's current release is). Neither
  leg is a hardcoded version string, so a floor bump or a new Go release needs no
  workflow edit. It also uploads coverage to Codecov. The `ci` job is
  an aggregate gate and is meant to be the **only** required status check —
  requiring the matrix jobs directly orphans the required context whenever a
  matrix value changes. Linting covers the four library modules — root,
  `adapter/redis`, `adapter/nats`, `prometheus` — matching `make lint-modules`;
  `examples/multinode` is only built, since it ships no tests and exists to
  catch API drift. `codecov.yml` ignores `examples/`, `cmd/` and the website,
  so coverage reflects library code only.

Releases have their own ordering constraints because this is a multi-module
repository, and Go module tags are permanent once the proxy has served them —
read [`RELEASING.md`](RELEASING.md) before tagging anything.

## Automated code review

Two bots review every PR; both are config-as-code and should stay in sync with
these conventions when they change:

- **CodeRabbit** — [`.coderabbit.yaml`](.coderabbit.yaml). Advisory only
  (`request_changes_workflow: false`); CI is the actual merge gate.
- **Greptile** — [`.greptile/`](.greptile/) (`config.json` for scoped rules and
  settings, `rules.md` for freeform style guidance, `files.json` for context
  files it should read). Also advisory.

Both carry per-path guidance roughly mirroring each other (root Go files vs.
`adapter/**` vs. `client.go`/`hub.go`'s no-close-send-channel rule vs.
`website/docs/**`'s version-marker requirement). If you change a convention
documented in this file that either bot enforces, update both configs in the
same PR — a stale bot rule actively misleads the next contributor it comments
on.

## Examples & load testing

- `examples/` — runnable examples (simple, chat, auth, metrics, multinode).
- `cmd/loadtest/` — real-WebSocket load generator: `make loadtest LOADTEST_ARGS="-scenario=fanout -clients=10000"`.

## Documentation website

The docs site lives on `main` in **`website/`** and is built with **Docusaurus**
(TypeScript, Biome for lint/format). It is published to GitHub Pages by
`.github/workflows/docs.yml` on every push to `main` that touches `website/` —
there is no manual deploy step and nothing to mirror onto another branch.

```bash
cd website
npm ci
npm start          # preview at localhost:3000
npm run check      # lint + typecheck + build — what the Docs workflow runs
```

`npm run lint` is `biome check`, which covers formatting as well as linting.
Biome has no Markdown support, so prose is linted separately and repo-wide with
`make lint-docs` (config: `.markdownlint-cli2.jsonc`).

**Versioning is by snapshot, not per release.** `website/docs/` is the
unreleased/current documentation; `website/versioned_docs/version-1.7/` is a
frozen snapshot of what a released version does.

- **Never edit `website/versioned_docs/`.** Changing a snapshot rewrites history
  for users still on that version. Snapshots are cut deliberately with
  `website/scripts/cut-version.mjs`; see `website/VERSIONING.md`.
- **Public API change** — update the matching page under `website/docs/` and mark
  the version inline rather than cutting a new snapshot: append `_1.8+_` to an
  API table cell, open a paragraph with `_Added in 1.8._`, add a trailing
  `// 1.8+` comment in a code block, or write `_Changed in 1.8._` plus one line
  on the previous behaviour.

Internal links are checked at build time (`onBrokenLinks: 'throw'`), so a
renamed page fails the Docs workflow rather than shipping a dead link. External
links are **not** checked.
