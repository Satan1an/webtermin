# Security policy

🌐 **English** · **[Русский](SECURITY.ru.md)**

## Pre-release audit (v0.1.0, 2026-05-16)

Before the first public tag the entire codebase was manually reviewed for the
common server-software risk classes, with automated scans on top.

### Tooling

- **`govulncheck ./...`** — clean against the published vulnerability database after bumping the toolchain to `go 1.25.10` (fixes 14 stdlib CVEs in `crypto/tls`, `crypto/x509`, `net`, `net/http`, `net/url`, `os`).
- **Grep audits**:
  - All `exec.Command` / `exec.CommandContext` call sites use argv slices, never `sh -c`.
  - All SQL statements use `?` placeholders — no string concatenation.
  - No hardcoded credentials, API keys, or tokens.
  - All file modes are intentional (`0o700` for secret dirs, `0o600` for cert/key + `authorized_keys`).

### Manual review highlights

| Area | Verdict |
| --- | --- |
| Authentication | Argon2id `(64 MiB, t=2, p=2)` meets the OWASP 2024 baseline; user-miss path runs a dummy hash to mitigate enumeration timing. |
| Session cookies | `HttpOnly` + `Secure` + `SameSite=Strict` on both set and clear. 256-bit random IDs. |
| CSRF | `X-CSRF-Token` header required on every state-changing request; constant-time compare. |
| Lockout | Per-IP failed-login counter with configurable threshold + cooldown; `Retry-After` returned on 429. |
| TLS | TLS 1.2 floor, self-signed P-256 cert auto-generated on first run, replaceable via config. |
| Security headers | HSTS 2y, strict CSP, X-Frame-Options DENY, nosniff, no-referrer, Permissions-Policy locked down. |
| Command exec | All callers (`chpasswd`, `useradd`, `userdel`, `journalctl`, `/bin/bash`) pass arguments via argv. Inputs validated by allowlist regexes before exec. |
| WebSocket | Same-origin enforced via `CheckOrigin`; missing/foreign Origin → upgrade refused. |
| Path safety | Every filesystem op in `internal/files/` goes through `SafePath` (absolute + cleaned; rejects `..`). |
| Audit log | Every mutating action recorded in SQLite with user, IP, action, target, outcome. |

### Known by-design trade-offs

- The panel process runs as **root** — required to manage system users, services, and arbitrary host files. Mitigated by Argon2id + 2FA + audit log + the action allowlist.
- The Docker compose file runs `privileged: true` with host `/etc`, `/home`, and `/run/dbus` bind-mounted — required to manage the host *from* a container.
- CSP allows `'unsafe-eval'` (Monaco editor workers) and `'unsafe-inline'` styles (Tailwind atomic CSS).
- Every panel user is currently admin; per-user RBAC is on the roadmap.

These are documented here so users can make an informed deployment decision, not because they're considered acceptable risks for every threat model.

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
