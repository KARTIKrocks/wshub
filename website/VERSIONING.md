# Documentation versioning

wshub ships often — eight minors between `v1.0.0` and `v1.7.0`, a span of four
and a half months. Versioned docs get unmaintainable fast if every release
snapshots the whole tree, so this site follows four rules.

## The rules

### 1. Snapshot when a release changes documented behaviour — not on every release

A snapshot exists to answer one question: *what was true before this release
broke it?* If nothing broke, the snapshot is a byte-identical copy of 15 files
that has to be maintained forever.

**The test:** does this release make an existing page **wrong for someone on the
previous version**? A changed default, changed semantics, a rename, a removal, a
deprecation — those get a snapshot. Purely additive releases do not.

`v1.7.0` is the worked example. It changed `DefaultConfig()` from
`AllowAllOrigins` to `AllowSameOrigin`, so every page describing the old default
became wrong for anyone still on `1.6`. That earned `version-1.7`. A release that
only adds an option does not — see rule 2 for what it gets instead.

Go makes this rule stronger than it would be elsewhere. Semver plus the Go
compatibility promise means additive minors leave the documented contract
intact, and a `v2` is a different import path
(`github.com/KARTIKrocks/wshub/v2`) with its own pkg.go.dev — a genuinely
different package. **Always snapshot at a major.**

### 2. Additive changes get a version marker, not a snapshot

The one real problem a reader on an older version has is the inverse of a
breaking change: they read about something that does not exist in their version
yet, and it does not compile. Snapshots are an expensive fix for that. A marker
is a cheap one, and it is a better answer — a `1.5` snapshot tells you what
existed, a marker tells you what upgrading buys you.

The convention is plain Markdown, so it needs no components and survives being
copied into a snapshot:

| Situation | Write |
| --- | --- |
| New row in an API table | append `_1.6+_` to the description cell |
| New option or behaviour in prose | open the paragraph with `_Added in 1.6._` |
| New member inside a code block | trailing `// 1.6+` comment |
| Behaviour that changed | `_Changed in 1.7._` plus one line on what it was before |

Markers use `MAJOR.MINOR` — `1.6`, not `v1.6.0` — so they match snapshot names
and stay greppable. Drop a marker once it names a version older than the oldest
live snapshot; by then everyone reading has it.

### 3. Snapshots are `MAJOR.MINOR`, never patch

Versions are `1.7`, `1.8`, `2.0`. Never `1.7.0`, never `v1.7`.

A patch release (`1.7.1`) that changes documented behaviour is **edited into the
existing `versioned_docs/version-1.7/` in place**. It does not get its own
snapshot. A patch by definition does not change the contract; if it did, the
docs were already wrong, and the fix belongs in the snapshot that is wrong.

`npm run cut-version` enforces the format; it rejects anything that isn't
`MAJOR.MINOR`, already exists, or is older than the current newest.

### 4. Only the newest 4 versions are built

`MAX_LIVE_VERSIONS` in `docusaurus.config.ts` caps how many snapshots get built
and indexed. Older ones stay in git — readable at their tag, restorable by
bumping the constant — but they don't cost build time or search index size.

This keeps build time flat as releases accumulate instead of growing linearly.

Four is generous under rule 1. Snapshotting every minor at the observed cadence
would burn the whole window in about six months; snapshotting only breaking ones
makes it last years. Raise it only if the reason is real support obligations,
not release count.

### 5. `docs/` is the future, not the present

| Directory | Serves | URL |
| --- | --- | --- |
| `docs/` | **Next** — unreleased, tracks `main` | `/docs/next/` |
| `versioned_docs/version-1.7/` | the current release | `/docs/` |

A reader who lands on `/docs/` sees released behaviour. Someone who wants what's
on `main` opens `/docs/next/`, which carries an "unreleased" banner.

This means **a PR that changes documented behaviour edits `docs/`**, not the
snapshot. The snapshot is frozen history.

## Release runbook

Every release starts the same way, and then rule 1 decides whether it ends
there.

### Every release

Make sure `docs/` describes the release accurately — everything merged into
`main` since the last release should already be reflected there — and that new
APIs carry their `_1.8+_` markers.

### If the release only adds

Nothing else to do. `docs/` becomes the new truth on the next deploy, the
markers tell readers on older versions what they need to upgrade to, and the
existing snapshot keeps serving `/docs/`.

Wait — that last part is the catch. `/docs/` serves the newest **snapshot**, so
an additive release does not reach the default URL until the next snapshot is
cut. That is the deliberate trade: readers see the last release whose behaviour
is fully described, and `/docs/next/` carries everything newer. If that gap gets
uncomfortably wide, that is the signal to cut a snapshot even without a breaking
change.

### If the release changes documented behaviour

```bash
cd website
npm run cut-version -- 1.8
npm run check
```

`versions.json`, `versioned_docs/version-1.8/`, and
`versioned_sidebars/version-1.8-sidebars.json` are created for you, `/docs/`
starts serving 1.8, and 1.7 moves into the version dropdown.

If the cut pushes a version out of the 4-version window, the script tells you
which one and prints the `git rm` to drop it for good.

### Patch releases

Never a snapshot. Edit `versioned_docs/version-<minor>/` directly, and mirror
the change into `docs/` if it still applies to `main`.

## Day-to-day

```bash
npm start          # dev server, builds Next + newest version only (fast)
npm run start:all  # dev server with every live version
npm run build      # production build, all live versions
npm run build:fast # build check, Next + newest only — what CI runs on PRs
npm run check      # lint + typecheck + fast build
```

`DOCS_FAST_BUILD=true` is what makes the first and fourth fast: it narrows the
build to the docs you're actually editing. Never set it for a production
deploy — the deploy workflow doesn't.

## Adding a page

1. Create `docs/<id>.md` with `id`, `title`, and `description` frontmatter.
2. Add the id to `sidebars.ts` under the right category.

The sidebar is explicit rather than autogenerated so ordering is a deliberate
choice and a new file can't silently rearrange the nav.

## What does not belong here

Type signatures, method sets, and struct fields. Those are generated from source
on [pkg.go.dev](https://pkg.go.dev/github.com/KARTIKrocks/wshub) and are always
correct; hand-copying them here creates a second source of truth that drifts.

These guides cover concepts, grouped overviews, configuration, and worked
examples — and link out for the exact signatures.
