# Security policy

## Supported versions

webtermin follows semantic versioning. While we're pre-1.0, only the latest
minor release receives security fixes.

| Version  | Supported          |
| -------- | ------------------ |
| `0.1.x`  | :white_check_mark: |
| `< 0.1`  | :x:                |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security reports.**

Use GitHub's private vulnerability reporting:

1. Open <https://github.com/Satan1an/webtermin/security/advisories/new>
2. Describe the impact, reproduction steps, and any suggested fix.

If GitHub's flow isn't an option, email **work@anubis.ooo** with subject
`[webtermin-security]`. Expect an initial reply within 72 hours.

## What's in scope

- Authentication or session bypass (any path that reaches an authed endpoint
  without a valid session).
- Authorization bypass (e.g., one user reading another's audit data or
  forging actions across sessions).
- Argument-injection or path-traversal in any module — even if it just
  produces an unexpected error, it's worth reporting.
- TLS, CSP, or cookie misconfigurations that weaken defaults.
- Container-escape paths in the documented Docker setup (beyond the
  inherent privilege the `privileged: true` mount-set grants).

## What's out of scope

- The fact that `docker-compose.yml` runs the container `privileged: true`
  and bind-mounts host paths — this is by design (the panel manages the
  host).
- The fact that the panel process runs as `root` — required for systemd /
  user / file management. Per-user RBAC is on the roadmap.
- Reports against test or example code in branches that aren't `main`.

## Disclosure timeline

Default: coordinated disclosure 90 days after the initial report, or upon
release of a fix — whichever comes first. We'll happily credit you in the
release notes if you'd like.
