# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] — 2026-05-17

### Added

- **API tokens for programmatic access**. Issue `wt_<32-byte-base64url>` tokens
  scoped to a specific role; use with `Authorization: Bearer wt_…`.
  - Tokens are 256-bit uniform random; only a SHA-256 hash is stored.
  - Plaintext is shown exactly once at creation — never recoverable.
  - The token's role *clamps* its owner's effective role for the request:
    a viewer token stays viewer even if its owner is later promoted to admin.
  - Token auth bypasses CSRF (no cookie to ride) but is otherwise treated
    identically to a session — including all RBAC checks and audit logging.
  - Role-cap on issue: a user cannot mint a token with privileges higher
    than their own.
  - Optional expiry in days (1–1825). Last-used timestamp is recorded.
  - Owner can revoke their own tokens; admins can revoke any.
- **`/api-tokens` page**: list, create (with role picker capped at the
  current role), one-time reveal dialog with copy-to-clipboard and an
  example curl command, revoke. Accessible to all authed roles —
  everyone sees their own tokens; admins see them all.

### Changed

- `requireSession` middleware renamed to `requireAuth` and now accepts
  either a session cookie or a bearer API token.
- `/api/auth/me` returns an empty `csrf_token` for token-authed requests
  (those clients don't need it).

### Tests

- 6 new server tests cover token creation, role-cap enforcement, scope
  clamping (viewer token can't perform operator actions), revocation,
  rejection of unknown tokens, and the non-owner-non-admin revoke guard.

## [0.2.0] — 2026-05-16

### Added

- **Role-based access control (RBAC)**. Three hierarchical roles:
  - `viewer` — read-only dashboard / services / files / Linux users / audit-less
  - `operator` — viewer + start/stop services, modify files, open web terminal
  - `admin` — operator + create/delete Linux users, manage panel users,
    read the audit log
  Existing v0.1 admin accounts migrate to `role = 'admin'` automatically.
- **Panel-users page** (admin-only) — list, create, change role, reset
  password, delete. Guard prevents removing or downgrading the last admin
  so the system stays recoverable.
- Frontend role badges in the user dropdown and disabled mutating buttons
  when the current user lacks the required role.
- `webtermin -version` flag now prints version, commit, and build date.
- Russian-language README (`README.ru.md`) and SECURITY policy
  (`SECURITY.ru.md`) with a language toggle from the English originals.

### Added — testing

- `go test ./...` now covers ~60 cases across `auth`, `files`, `users`,
  `systemd`, and `server`. Race detector enabled. Highlights:
  - End-to-end auth flow via `httptest` (setup → login → CSRF guard →
    logout → re-auth-rejected).
  - RBAC matrix: viewer can't mutate, operator can't manage users, admin
    can; last-admin guard blocks self-demotion.
  - Argon2id roundtrip + salt uniqueness, TOTP code validation, login
    lockout, per-IP isolation, expired session rejection.
  - `files.SafePath` traversal/sibling-prefix attacks; `parseKeyLine` no
    longer accepts empty blobs.
  - `systemd.ValidUnitName` against shell-metachar adversaries.
- CI gains a `go test -race -count=1 ./...` step.

### Changed

- `store.CreateUser` now takes a `role` argument. Callers updated.
- `gosec` strict gate excludes the noisy taint-analysis siblings G703
  (the new G304-equivalent) on top of the existing G104/G301/G302/G304.

### Fixed

- `parseKeyLine("   ")` previously returned a "valid" key with an empty
  blob and a fingerprint of the empty string. Now requires non-empty type,
  blob, and decoded raw bytes. Discovered by the new test suite.

## [0.1.0] — 2026-05-16

### Security

- Pre-release security audit performed: manual review of auth/CSRF/CSP, plus
  `govulncheck`, argv-exec, SQL-parameterization and path-safety sweeps. Full
  summary in [SECURITY.md](SECURITY.md#pre-release-audit-v010-2026-05-16).
- Bumped Go toolchain directive to `go 1.25.10` — fixes 14 stdlib CVEs in
  `crypto/tls`, `crypto/x509`, `net`, `net/http`, `net/url`, `os`.
- PTY shell is now allowlisted against `/etc/shells` before exec, so a
  tampered config can't launch arbitrary binaries through the terminal endpoint.
- `Retry-After` header returned on `429 Too Many Requests` from login lockout.
- WebSocket `CheckOrigin` now hard-refuses missing/foreign Origin (was previously
  permissive on missing Origin).
- CI pipeline gains four security checks that block merges:
  - `gofmt` formatting
  - `staticcheck` (full Go linter set)
  - `gosec` at HIGH severity (Go SAST)
  - `govulncheck` (Go stdlib + dependency CVE scan)
  - `gitleaks` (commit/history secret scanner)
- Dependabot configured for `gomod`, `npm`, GitHub Actions, and Docker — weekly
  grouped PRs.

### Added

- `webtermin -version` flag — prints version, commit, and build date (populated
  at build time via `-ldflags`).

### Added
- Initial public release.
- HTTPS server with self-signed cert auto-generation on first run.
- First-run admin setup wizard.
- Argon2id password hashing; session cookies (HttpOnly/Secure/SameSite=Strict); CSRF tokens; optional TOTP 2FA.
- Per-IP rate-limit and lockout on failed logins.
- Audit log in SQLite — every mutating action recorded with user, IP, target, outcome.
- **Dashboard** module — live CPU/memory/network/disk metrics via WebSocket, system info, uptime.
- **Services** module — list/start/stop/restart/enable systemd units via D-Bus; live `journalctl` tail.
- **Users** module — create/delete Linux users, set passwords, manage `~/.ssh/authorized_keys`.
- **Files** module — browse with safe-path enforcement, upload/download, Monaco editor, chmod, mkdir, delete.
- **Terminal** module — in-browser PTY via xterm.js bridged over WebSocket.
- React 18 + Tailwind + shadcn-style UI with dark theme, framer-motion transitions, recharts.
- Single static Go binary with embedded SPA via `go:embed`.
- Docker multi-stage Dockerfile and `docker-compose.yml` (privileged host control).
- `.deb` packaging for `linux/amd64` and `linux/arm64` via GoReleaser + nfpm.
- GitHub Actions: CI on PR/push (build + cross-build + go vet) and Release on tag (full GoReleaser flow).

[Unreleased]: https://github.com/Satan1an/webtermin/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.3.0
[0.2.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.2.0
[0.1.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.1.0
