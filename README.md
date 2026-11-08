<div align="center">

<img src="docs/banner.svg" alt="webtermin — self-hosted server control panel" width="100%">

<h3>A beautiful, secure, self-hosted alternative to Webmin for a single Linux server.</h3>

[![CI](https://github.com/Satan1an/webtermin/actions/workflows/ci.yml/badge.svg)](https://github.com/Satan1an/webtermin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Satan1an/webtermin?display_name=tag&sort=semver&color=00DC82)](https://github.com/Satan1an/webtermin/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-22d3ee.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platforms](https://img.shields.io/badge/linux-amd64%20%7C%20arm64-555)](#install)

🌐 **English** · **[Русский](README.ru.md)**

</div>

---

**webtermin** is a single static Go binary with an embedded React SPA. It runs HTTPS-only out of the box, ships with first-run admin setup, Argon2id passwords, optional TOTP 2FA, CSRF-protected sessions, and a full audit log. Every system action goes through a typed allowlist — **no shell strings built from user input, ever**.

## Highlights

| | |
|---|---|
| ⚡ **One binary** | Static Go executable (~14 MB) with the React SPA embedded via `go:embed`. No runtime, no separate static server. Same binary in `.deb` / `.rpm` / `.apk`. |
| 🔐 **Security-first** | TLS by default · Argon2id · HttpOnly/Secure/Strict cookies · CSRF tokens · strict CSP · rate-limit + lockout · optional TOTP 2FA · RBAC (viewer / operator / admin) · API tokens with role scoping · OIDC SSO. |
| 📦 **Can't brick the box** | Package installs are dry-run, checked and confirmed. Core packages, the running kernel, the bootloader and the live network stack are never removed — see [how](#package-safety). |
| 📜 **Audit log** | Every mutating action recorded in SQLite — who, what, when, target, outcome, IP. Surfaced in-UI under `/audit`. |
| 🧩 **13 modules** | See [docs/modules.md](docs/modules.md). Systemd · Docker (Portainer-grade) · Compose stacks · cron · Linux users + SSH keys · files (Monaco) · PTY terminal · packages (apt/dnf, dry-run plans) · firewall (ufw) · network (nmcli) · WireGuard · backups · panel users + tokens. |
| 🎨 **Modern UI** | React + Tailwind + shadcn/Radix · recharts · Framer Motion · dark by default · responsive. |
| 🏗️ **ARM-ready** | Cross-compiled for `linux/amd64` and `linux/arm64` (OrangePi, Raspberry Pi 4/5, ARM cloud VMs). |
| 📚 **Documented** | Per-module reference, complete API, OIDC + non-root setup walkthroughs, contributor guide — all under [`docs/`](docs/). |

## Screenshots

<p align="center">
  <img src="docs/images/dashboard.png" alt="Dashboard with live CPU, memory and network charts" />
</p>

<table>
<tr>
<td width="50%"><img src="docs/images/docker.png" alt="Docker containers with per-container actions" /><br><sub><b>Docker</b> — containers, images, networks, volumes, exec, logs</sub></td>
<td width="50%"><img src="docs/images/terminal.png" alt="Web terminal" /><br><sub><b>Terminal</b> — PTY over WebSocket, as the mapped Linux user</sub></td>
</tr>
<tr>
<td><img src="docs/images/packages.png" alt="Package dry-run dialog warning that NetworkManager may take over the network" /><br><sub><b>Packages</b> — every install is a dry run first, with warnings</sub></td>
<td><img src="docs/images/firewall.png" alt="ufw firewall rules" /><br><sub><b>Firewall</b> — ufw status and rules</sub></td>
</tr>
<tr>
<td><img src="docs/images/stacks.png" alt="Docker Compose stacks" /><br><sub><b>Stacks</b> — Compose deployments</sub></td>
<td><img src="docs/images/audit.png" alt="Audit log including a blocked package removal" /><br><sub><b>Audit log</b> — every action, including refused ones</sub></td>
</tr>
<tr>
<td><img src="docs/images/cron.png" alt="Cron entries for root" /><br><sub><b>Cron</b> — per-user crontabs</sub></td>
<td><img src="docs/images/users.png" alt="Linux users" /><br><sub><b>Users</b> — Linux accounts and SSH keys</sub></td>
</tr>
</table>

## How it works

<p align="center">
  <img src="docs/architecture.svg" alt="Browser or API client talks HTTPS to the webtermin binary, which passes every request through edge, authentication, RBAC, a typed allowlist and safety checks before touching systemd, Docker, apt, ufw, nmcli, WireGuard, cron, users, files or a PTY" width="100%">
</p>

## Install

### Debian / Ubuntu / OrangePi / Raspberry Pi (.deb)

Grab the package for your architecture from the [latest release](https://github.com/Satan1an/webtermin/releases/latest):

```bash
# Latest release tag; ARCH auto-detects amd64 / arm64.
VERSION=$(curl -fsSL https://api.github.com/repos/Satan1an/webtermin/releases/latest \
  | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p')
ARCH=$(dpkg --print-architecture)
curl -fsSL -o webtermin.deb \
  "https://github.com/Satan1an/webtermin/releases/download/v${VERSION}/webtermin_${VERSION}_linux_${ARCH}.deb"
sudo apt install ./webtermin.deb
```

The .deb installs to `/usr/bin/webtermin`, drops a systemd unit at `/lib/systemd/system/webtermin.service`, creates `/var/lib/webtermin` for data and `/etc/webtermin/config.yaml` as a conffile (your local edits are preserved on upgrade), and starts the service.

Open **`https://<host>:8443`** to complete first-run setup. The setup page asks for a one-time **setup token** so nobody else can claim a fresh install first; it is printed to the journal:

```bash
sudo journalctl -u webtermin | grep setup_token
```

### Why no Docker image?

webtermin manages the host itself — systemd, users, packages, firewall, files. A container doing that needs `privileged`, the host PID namespace and the host filesystem, so it isolates nothing, and every module would have to reach back out to the host. Install it natively (`.deb` / `.rpm` / `.apk`) instead. The Docker *module* — managing containers on the host — works as usual.

## Build from source

```bash
make build         # native binary for this host
make build-arm64   # cross-compile aarch64 (for OrangePi / RPi)
```

Requires Go ≥ 1.26 and Node ≥ 22 (LTS).

## Configuration

The default config lives at `/etc/webtermin/config.yaml` (or `./config.yaml` when running standalone). All keys are commented in [`config.example.yaml`](config.example.yaml). The values you'll touch most often:

| Key | Default | Notes |
| --- | --- | --- |
| `server.listen` | `0.0.0.0:8443` | Bind `127.0.0.1` if you put nginx/Caddy in front. |
| `server.tls_cert` / `tls_key` | _empty_ | Empty → self-signed cert auto-generated into `data_dir/tls/`. |
| `data_dir` | `/var/lib/webtermin` | Holds the SQLite DB, sessions, and generated certs. |
| `security.session_ttl_min` | `240` | Session lifetime in minutes. |
| `security.require_2fa` | `false` | Force TOTP enrollment for every user. |
| `security.max_login_attempts` | `5` | Per-IP failure count before lockout. |
| `terminal.default_shell` | _empty_ | Fallback shell for the web terminal. |

After editing: `sudo systemctl restart webtermin`.

## Documentation

| | |
|---|---|
| **Modules reference** — [`docs/modules.md`](docs/modules.md) | Every module, what it does, which role can read / write, audit-log namespaces. |
| **HTTP API** — [`docs/api.md`](docs/api.md) | Complete endpoint list grouped by module. The reference for anyone scripting webtermin via API tokens. |
| **OIDC SSO setup** — [`docs/oidc-setup.md`](docs/oidc-setup.md) | Concrete walkthrough using Authentik as the worked example. |
| **Non-root deployment** — [`docs/non-root-setup.md`](docs/non-root-setup.md) | sudoers + docker-group recipe for least-privilege deployments. |
| **Contributing** — [`docs/contributing.md`](docs/contributing.md) | Repo layout, build / test / lint, the pattern for adding a new module. |

## Security model

- **Transport** — HTTPS only, TLS 1.2+, HSTS 2y, X-Frame-Options DENY, strict Content-Security-Policy.
- **Auth** — Argon2id (64 MiB, t=2, p=2). Session IDs are 256-bit, random, HttpOnly + Secure + SameSite=Strict, and stored only as SHA-256 hashes. TOTP codes can't be replayed. First-run setup needs a one-time token from the journal. CSRF token required on every mutating request. Optional per-user TOTP 2FA. Rate-limit + lockout on failed logins. RBAC with three hierarchical roles. API tokens with per-token role scoping. Optional OIDC SSO sign-in.
- **System actions** — Every action lives behind a typed allowlist. Unit names match `^[A-Za-z0-9@_.\-:\\]+\.(service|socket|...)$`; usernames match `^[a-z_][a-z0-9_-]{0,31}$`; file paths must be absolute, cleaned, and free of `..`. **No shell strings constructed from user input** — `useradd`, `chpasswd`, `journalctl`, `ufw`, `wg`, `nmcli`, `apt-get` etc. are invoked with argv slices via `os/exec`.
- **Packages** — dry run, safety checks and explicit confirmation before anything runs; see [Package safety](#package-safety).
- **Audit** — every mutating action writes to `audit_log` (user, IP, action, target, outcome, detail). Surfaced under `/audit` in the UI.
- **Process** — runs as root by default; for least-privilege deployments see [docs/non-root-setup.md](docs/non-root-setup.md). systemd unit hardening: `ProtectKernelTunables=true`, `ProtectKernelModules=true`.

### Roles

<p align="center">
  <img src="docs/roles.svg" alt="viewer looks, operator operates without root-equivalent power, admin equals root" width="100%">
</p>

### Package safety

<p align="center">
  <img src="docs/package-safety.svg" alt="Dry run, block dangerous removals, warn, admin confirms the plan, guarded run, audit" width="100%">
</p>

### Audit history

A manual review + `govulncheck` sweep was performed before tagging v0.1.0
(2026-05-16): grep audit of `exec.Command` argv usage, SQL parameterization,
file modes, cookie attributes; deep review of auth, CSRF, WebSocket origin,
path safety, and command exec. Results and the by-design trade-offs are
documented in [SECURITY.md](SECURITY.md#pre-release-audit-v010-2026-05-16).

Each subsequent release (v0.2 → v0.9) added unit tests for the new module's
validators. A second full review before v0.10.0 (2026-11-08) closed setup
takeover, session-at-rest, TOTP replay, RBAC root-equivalence and container
host-escape issues — see [SECURITY.md](SECURITY.md#security-review-2026-11-08).

`go test -race`, `staticcheck`, `gosec` (HIGH), `govulncheck` and `gitleaks`
run on every CI build and weekly.

Found a vulnerability? See [SECURITY.md](SECURITY.md).

## Roadmap

Initial v0.1 → v0.9 roadmap is **complete**. Highlights of what shipped:

- ✅ RBAC (viewer / operator / admin) + last-admin guard
- ✅ API tokens with role-clamping at issue time
- ✅ OIDC SSO (Authentik / Authelia / Keycloak / Auth0 / …)
- ✅ Cron, firewall (ufw), network (nmcli), WireGuard, packages (apt/dnf)
- ✅ Portainer-grade Docker module + Compose stacks
- ✅ Backups (tar.gz of `/etc/webtermin` + `/var/lib/webtermin` + custom paths)
- ✅ Non-root mode (sudoers + docker group)
- ✅ `.deb` / `.rpm` / `.apk` packages
- ✅ Russian-language README + SECURITY
- ✅ v0.10: security review fixes, one-click install of missing tools, package safety checks

Post-1.0 ideas (gather user feedback first):

- [ ] Backup scheduling via systemd timers
- [ ] Wi-Fi management (`nmcli dev wifi …`)
- [ ] 24h Dashboard history (rrdtool-style)
- [ ] Email / webhook alerts on selected audit events
- [ ] Compose: image build from `build:` directive, healthcheck, secrets
- [ ] Docker: registry credentials, events stream, volume content browser

PRs welcome — see [docs/contributing.md](docs/contributing.md). Open an
issue first for anything non-trivial.

## License

[MIT](LICENSE) © 2026 Satan1an
