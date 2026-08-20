# Contributing to wshub

Thanks for your interest in contributing!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/wshub.git`
3. Create a branch: `git checkout -b fix/short-description` (see
   [Git conventions](#git-conventions) for the naming)
4. Make your changes
5. Run checks: `make ci`
6. Push and open a pull request

## Development

### Prerequisites

- Go 1.27+
- golangci-lint v2

### Running Tests

```bash
make test          # run root-module tests with the race detector
make test-modules  # run tests for adapter/redis, adapter/nats, prometheus
make bench         # run benchmarks
make lint          # run linter on the root module
make vuln          # govulncheck on the root module
make vuln-modules  # govulncheck on the nested modules
make tidy          # go mod tidy on the root module
make tidy-modules  # go mod tidy on the nested modules
make ci            # run all checks across every module
```

This repository is multi-module: the adapters and the Prometheus collector each
have their own `go.mod`, so the root module's `./...` does not reach them. Use
`make ci` (or `make all`) to cover everything the way CI does.

### Documentation

The site at <https://kartikrocks.github.io/wshub/> is built with Docusaurus and
lives in [`website/`](website/). It needs Node 20+.

```bash
cd website
npm ci
npm start          # preview at localhost:3000, hot-reloads on save
npm run check      # lint + typecheck + build — what the Docs workflow runs
```

`npm run lint` is `biome check`, which covers formatting as well as linting;
`npm run lint:fix` applies both. Biome has no Markdown support, so the prose
itself is linted separately, from the repository root:

```bash
make lint-docs      # structural lint for every .md in the repo
make lint-docs-fix  # apply the fixes it can make automatically
```

That covers the root `README.md`, `SECURITY.md`, and this file too, not just
the site. Rules and the reasoning behind each exception are in
[`.markdownlint-cli2.jsonc`](.markdownlint-cli2.jsonc).

Edit `website/docs/`. **Do not edit `website/versioned_docs/`** — those are
frozen snapshots of what a released version does, and changing one rewrites
history for users still on it.

The site is versioned by snapshot, not per release, so a new API needs a version
marker in the prose rather than a new snapshot:

| Situation | Write |
| --- | --- |
| New row in an API table | append `_1.8+_` to the description cell |
| New option or behaviour in prose | open the paragraph with `_Added in 1.8._` |
| New member inside a code block | trailing `// 1.8+` comment |
| Behaviour that changed | `_Changed in 1.8._` plus one line on what it was before |

[`website/VERSIONING.md`](website/VERSIONING.md) has the full runbook, including
when a release does earn a snapshot. Read it before cutting one.

### Code Style

- Follow standard Go conventions
- Run `gofmt` and `goimports` before committing
- All exported types and functions must have doc comments
- Keep test coverage high for new code

## Git conventions

**Branches** are named `type/kebab-slug`, using the same type as the commit:

```text
fix/close-with-code-send-channel-race
ci/test-submodules
docs/docusaurus-website
```

**Commits** follow [Conventional Commits](https://www.conventionalcommits.org/):
`type(optional-scope): imperative summary`, no trailing period.

```text
fix(examples): drop the redundant adapter Close from multinode
ci: gate on a single aggregate check, trim the root Go matrix
fix(security): default CheckOrigin to same-origin and surface rejections
```

| Type | Use for |
| --- | --- |
| `feat` | new public API or user-facing capability |
| `fix` | bug fixes; security fixes are `fix(security):` |
| `docs` | documentation and the docs site |
| `test` | tests only |
| `perf` | performance work with no behaviour change |
| `refactor` | internal restructuring with no behaviour change |
| `ci` | workflows and CI configuration |
| `build` | Makefile, module files, tooling |
| `chore` | everything else; dependency bumps are `chore(deps):` |
| `revert` | reverting a previous commit |

**Pull requests** are squash-merged, so the **PR title becomes the commit
message on `main`** and must itself be a valid Conventional Commit. Individual
commits on your branch are working state — they get collapsed, so there is no
need to rebase them into a tidy sequence before review.

Every change goes through a pull request; nothing is pushed directly to `main`.

## Pull Requests

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation if the public API changes
- Ensure `make ci` passes before requesting review
- Add a `CHANGELOG.md` entry for anything user-facing. Write it for someone
  deciding whether to take the upgrade: what changed, who is affected, and what
  they need to do. The changelog is hand-written on purpose — it is the
  migration guide, not a list of commit subjects.

## Releasing

Maintainers only. This is a multi-module repository, so releases have a required
ordering — the root module must be published before any submodule can pin it,
and submodule tags are path-prefixed (`adapter/redis/v0.2.3`). Tags are permanent
once the Go module proxy has served them. See [`RELEASING.md`](RELEASING.md).

## Reporting Issues

- Use GitHub Issues
- Include Go version, OS, and a minimal reproduction

## License

By contributing you agree that your contributions will be licensed under the MIT License.
