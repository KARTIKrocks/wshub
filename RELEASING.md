# Releasing

wshub is a multi-module repository, and the modules depend on each other. That
makes release **ordering** load-bearing: tag them in the wrong order and you
publish a module that nothing can build.

> **Tags are permanent.** The Go module proxy caches module content on first
> fetch and never re-reads it. Deleting a tag and re-pushing it does not undo a
> mistake — `proxy.golang.org` keeps serving the original bytes, forever. If you
> publish a broken version, the only remedy is to publish another one. Check
> before you push, not after.

## Module graph

```text
github.com/KARTIKrocks/wshub          (root)
  ├── wshub/adapter/redis             pins wshub
  ├── wshub/adapter/nats              pins wshub
  ├── wshub/prometheus                pins wshub
  └── wshub/examples/multinode        pins wshub AND adapter/redis
```

All four pin the root module directly; `prometheus` is not a prerequisite for
anything. The only second-order dependency is `examples/multinode`, which also
pins `adapter/redis` and therefore has to come after it.

Release strictly top-down: **root → adapters and prometheus → examples**. A
submodule can only pin a wshub version that is already published.

Tags for submodules are **path-prefixed**. The tag that releases
`adapter/redis` v0.2.3 is `adapter/redis/v0.2.3`, not `v0.2.3` — a bare `v0.2.3`
would be read as a release of the *root* module.

## Choosing the version

The root module is on `v1`. Within that line:

| Change | Bump |
| --- | --- |
| New public API, backwards compatible | minor |
| Bug fix, no behaviour change for correct usage | patch |
| **Any change to runtime behaviour**, including a security default | **minor** |
| Breaking API change (would not compile) | needs `v2` — see below |

The third row is the one that catches people. A fix that starts rejecting
traffic the previous release accepted is not a patch, even though it compiles
cleanly, because patch releases get merged without being read. v1.7.0 is the
worked example: the `AllowSameOrigin` default fixed cross-site WebSocket
hijacking and deliberately began rejecting cross-origin upgrades, so it shipped
as a minor with a loud changelog entry. See [`SECURITY.md`](SECURITY.md).

A genuinely breaking API change requires a `/v2` module path — a new directory
tree, a new import path for every user, and a parallel maintenance burden.
Exhaust the alternatives first: add a new function rather than changing a
signature, and deprecate rather than delete.

The adapters and `prometheus` are on their own `v0.x` line and version
independently. Keep them independent — it is what lets an adapter fix reach
users who are not ready for the current wshub minor.

## Releasing the root module

1. **Confirm `main` is green.** The `ci` aggregate check must be passing on the
   commit you intend to tag.

2. **Write the changelog entry.** Move the pending section in `CHANGELOG.md`
   under a `## [X.Y.Z] - YYYY-MM-DD` heading. Write it for someone deciding
   whether to take the upgrade — what changed, who is affected, who isn't, and
   the code they need to change. For anything breaking, lead with it.

3. **Verify everything, from a clean tree:**

   ```bash
   make all          # vet + lint + test + build, every module
   make tidy-check   # fails if any go.mod/go.sum is untidy, leaving no diff
   make lint-docs    # Markdown across the repo
   ```

4. **Update the docs for the new version.** Version markers in `website/docs/`
   (`_1.8+_`, `_Added in 1.8._`, `// 1.8+`, `_Changed in 1.8._`) go in *before*
   the tag. Whether this release also earns a frozen snapshot under
   `versioned_docs/` is a separate decision — [`website/VERSIONING.md`](website/VERSIONING.md)
   has the criteria and the `cut-version.mjs` runbook.

5. **Tag and push:**

   ```bash
   git tag -a v1.8.0 -m "v1.8.0"
   git push origin v1.8.0
   ```

6. **Wait for the proxy** before touching the submodules. The version is not
   usable until `proxy.golang.org` has fetched it; this warms it and confirms:

   ```bash
   GOPROXY=proxy.golang.org go list -m github.com/KARTIKrocks/wshub@v1.8.0
   ```

   A 404 here means step 7 will fail — wait and retry rather than working around
   it with a `replace`.

7. **Publish the GitHub release** from the tag. Release notes are
   auto-generated; link to the `CHANGELOG.md` section for the detail.

## Releasing a submodule

Only after the root version it pins is live.

1. **Bump the pin** in each submodule that needs the new core:

   ```bash
   cd adapter/redis && go mod edit -require=github.com/KARTIKrocks/wshub@v1.8.0
   ```

2. **Tidy, ignoring the workspace.** This step is the real check:

   ```bash
   make tidy-modules
   ```

   `go mod tidy` deliberately ignores `go.work`, so it resolves the pin from the
   proxy — exactly what a consumer gets. If you pinned a version that isn't
   published, it fails here instead of after you tag.

3. **Test against the published pin**, not the working tree:

   ```bash
   cd adapter/redis && GOWORK=off go test -race ./...
   ```

   `make test-modules` uses the workspace and would resolve wshub from your
   local checkout, which hides a bad pin.

4. **Commit the bump through a PR** (`chore(deps): bump wshub to v1.8.0 in every
   nested module`), let CI pass, and merge.

5. **Tag from `main` after the merge**, path-prefixed:

   ```bash
   git tag -a adapter/redis/v0.2.3 -m "adapter/redis v0.2.3"
   git push origin adapter/redis/v0.2.3
   ```

6. **Repeat for `examples/multinode` last** — it pins both wshub and
   `adapter/redis`, so it can only be updated once both are published. It is not
   a released module (nobody imports it), so it needs no tag; the pin bump alone
   keeps the example building against what users actually get.

## After the release

- Confirm [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub) shows
  the new version. It can lag the proxy by a few minutes.
- Confirm the Docs workflow deployed — the site is published from `main` on
  every `website/**` change, so a docs update merged before the tag is already
  live.
- Open the pending `## [Unreleased]` section in `CHANGELOG.md` for the next
  cycle.
