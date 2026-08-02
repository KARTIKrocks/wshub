# Security Policy

wshub terminates untrusted WebSocket upgrades, so its defaults and its handling
of hostile input are treated as part of the API rather than as configuration
details.

## Reporting a vulnerability

Please report privately through
[GitHub Security Advisories](https://github.com/KARTIKrocks/wshub/security/advisories/new)
rather than opening a public issue, so a fix can ship before the details are
public.

Useful things to include, if you have them: the affected version, whether the
issue is reachable from an unauthenticated connection, and a minimal `Hub`
configuration that reproduces it.

You can expect an acknowledgement within 7 days. If a report is confirmed, the
advisory is published together with the release that fixes it, and you will be
credited unless you ask otherwise.

## Supported versions

Security fixes ship on the latest minor of each module. Older minors do not
receive them, so the remedy for a reported issue is always to upgrade within
`v1`.

A fix that changes behaviour ships as a **minor** bump, never a patch. This
matters more than it sounds: patch releases are the ones that get merged without
being read, so a fix that starts rejecting traffic the previous release accepted
cannot go out that way. The consequence is that such a fix has nowhere to live
except the new minor — there is no older release it could be backported to
without misrepresenting what it does.

The practical reading, if you are pinned to an older version: upgrading within
`v1` always compiles, because the Go compatibility promise holds across the
line. It does not always behave identically. Read the changelog entry for the
release before taking it — v1.7.0 is the worked example, where the fix for
cross-site WebSocket hijacking deliberately began rejecting cross-origin
upgrades that v1.6.1 allowed.

| Version | Supported |
| --- | --- |
| Latest `v1` minor | Yes |
| Anything older | No — upgrade |

The adapters (`adapter/redis`, `adapter/nats`) and `prometheus` are versioned
independently, and the same rule applies to each at its own latest minor. That
independence is deliberate: an adapter fix can reach users who are not ready for
the current wshub minor, which is how the v0.2.2 adapter fixes shipped to
v1.6.1 users without forcing the v1.7.0 origin change on them.

## Scope

In scope — issues reachable from a connection the server did not choose to
trust:

- Origin validation bypass, or any path that accepts an upgrade the configured
  `CheckOrigin` should have rejected
- Panics reachable from remote input, including malformed frames and malformed
  adapter payloads
- Resource exhaustion that a single client can trigger without cooperation from
  the server — unbounded buffering, goroutine leaks, missing limit enforcement
- Data races that can corrupt hub or room state
- Authentication or authorization bypass in the middleware chain

Out of scope:

- Misconfiguration that disables a protection deliberately, such as setting
  `WithCheckOrigin(wshub.AllowAllOrigins)`
- Denial of service that requires privileged network position or client
  cooperation
- Findings in `examples/`, which is illustrative code and not part of the
  supported surface
- Vulnerabilities in third-party dependencies — please report those upstream.
  Dependabot tracks them here and `govulncheck` gates CI on the ones this code
  actually reaches.

## Secure defaults

`DefaultConfig()` is intended to be safe to deploy as-is. Notably, since v1.7.0
it uses `AllowSameOrigin` rather than `AllowAllOrigins`; the previous default
accepted an upgrade from any origin, which left servers built on it open to
cross-site WebSocket hijacking. See [CHANGELOG.md](CHANGELOG.md) for the
migration.

Configuration that carries security weight — origin checks, message size caps,
rate limits, and read/write deadlines — is documented under
[Configuration](https://kartikrocks.github.io/wshub/docs/configuration) and
[Limits](https://kartikrocks.github.io/wshub/docs/limits).

## How this is checked

| Tool | Covers |
| --- | --- |
| [CodeQL](.github/workflows/codeql.yml) | Bug patterns in this repository's own Go code, scanned per module, on every push and pull request plus a weekly re-scan |
| [`govulncheck`](.github/workflows/ci.yml) | Known advisories in dependencies, filtered to those reachable from this code's call graph, run per module |
| [Dependabot](.github/dependabot.yml) | Dependency updates for all four Go modules and the docs site, plus the GitHub Actions themselves |

Each of the three checks runs separately against the root module,
`adapter/redis`, `adapter/nats`, and `prometheus`, because a scan started from
the root stops at the nested `go.mod` boundaries and would otherwise miss the
adapters entirely.
