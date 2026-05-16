# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-05-16

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

[Unreleased]: https://github.com/Satan1an/webtermin/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.1.0
