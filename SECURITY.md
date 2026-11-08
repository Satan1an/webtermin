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
- CSP allows `'unsafe-eval'` (Monaco editor workers) and `'unsafe-inline'` styles (Tailwind atomic CSS).
- Roles (viewer / operator / admin) are enforced, but because the panel runs as root the line is drawn at *root-equivalence*: anything that would give a shell or file access as root — the file manager, a root terminal, privileged / host-network / host-bind containers, other users' crontabs, backups — is admin-only. See the 2026-11 review below.

These are documented here so users can make an informed deployment decision, not because they're considered acceptable risks for every threat model.

## Security review (2026-11-08)

A review of the role model found that the RBAC described above was weaker in
practice, because the panel acts as root on behalf of every role. Fixed:

| Issue | Fix |
| --- | --- |
| A **viewer** could read/download any file as root — `/etc/shadow`, private keys, and the panel DB, whose sessions were stored in plaintext → hijack an admin session. | Files module is admin-only; sessions are stored as SHA-256 hashes like API tokens, so the DB holds no usable cookie. |
| The **terminal** fell back to the panel process user (root) when a panel user had no same-named Linux account. | Fails closed for non-admins; a non-admin is also never given a uid-0 account. Shells now keep the user's own supplementary groups and `USER`/`LOGNAME`. |
| An **operator** was root-equivalent via `privileged` / host-network / host-bind containers and stacks, `exec` into such containers, and editing root's crontab. | Those are admin-only (`HostEscape` / `InspectHostEscape`); operators manage only their own non-root crontab. |
| Secret-bearing reads (container inspect + logs, service journals, cron, full stack compose text, backup download) were open to viewers. | Operator+ (backup download: admin); stack list redacts `compose` for viewers. |
| **First-run setup** was claimable by whoever reached a fresh install first; count-then-create race. | Requires a one-time token printed to the service log; serialised with a mutex. |
| A TOTP code could be replayed within its window. | Each user's last accepted time-step is remembered; reuse is rejected. |
| Built with Go 1.25 (no longer receives security fixes); 17 known CVEs in required modules. | Go ≥ 1.26 (CI/releases build with the newest stable Go), dependencies updated; CI also runs weekly so `govulncheck` catches newly published CVEs. |

## Supported versions

webtermin follows semantic versioning. While we're pre-1.0, only the latest
minor release receives security fixes.

| Version  | Supported          |
| -------- | ------------------ |
| `0.11.x` | :white_check_mark: |
| `< 0.11` | :x:                |

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

## What's out of scope

- The fact that the panel process runs as `root` — required for systemd /
  user / file management.
- Reports against test or example code in branches that aren't `main`.

## Disclosure timeline

Default: coordinated disclosure 90 days after the initial report, or upon
release of a fix — whichever comes first. We'll happily credit you in the
release notes if you'd like.
