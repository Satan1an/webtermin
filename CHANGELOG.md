# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.9.1] — 2026-05-17

### Fixed

- Release pipeline now ships the Docker image. v0.9.0's release run
  failed at the docker-build stage because `config.example.yaml` wasn't
  in the goreleaser-prepared build context; the `.deb`/`.rpm`/`.apk`
  packages were built but never uploaded since the workflow exited.
- Added `extra_files: [config.example.yaml]` to both docker stanzas
  so the file is staged into goreleaser's tmp build dir alongside the
  binary.

No application-level changes — v0.9.0 + v0.9.1 are functionally
identical, v0.9.1 just gets through the release pipeline cleanly.

## [0.9.0] — 2026-05-17

### Added — Network module (`/network`, admin-only)

- `internal/network` wraps NetworkManager's `nmcli`. On hosts without
  it (minimal containers, systemd-networkd-only setups) the module
  reports `available: false` and the UI shows an install hint instead
  of erroring out.
- Interface list with state badge, MAC, IPv4 addresses + gateway,
  IPv6 addresses, DNS servers, and IPv4 method (auto / manual).
  Loopback is filtered out — it's noise.
- Per-interface edit dialog: toggle Automatic (DHCP) vs Static. For
  static mode: address (x.x.x.x/yy), gateway, comma- or
  space-separated DNS list. Settings are applied immediately via
  `nmcli con up` AND persisted via `nmcli con mod`, so they survive
  reboot.
- Hostname read via `hostname` and changed via
  `hostnamectl --static set-hostname` (works on every systemd distro).
- All inputs go through tight regexes before exec — interface name
  follows IFNAMSIZ, IPv4 must include CIDR for static, hostnames are
  RFC 1123. Each command runs via argv slices, never shell strings.
- Confirmation dialog with explicit warning ("don't edit the
  interface you're connected through") because a wrong gateway is
  the easiest way to lock yourself out of a remote host.
- Audited as `network.{static,dhcp,dns,hostname}`.

### Tests

- `network.ValidIface` (kernel IFNAMSIZ rules)
- `network.ValidIPv4WithCIDR` (rejects bare IPs and IPv6)
- `network.ValidIP` (accepts v4 and v6)
- `network.ValidHostname` (RFC 1123)
- `splitNmcliFields` handles `\:` escapes inside CONNECTION names

## [0.8.0] — 2026-05-17

The three big remaining roadmap items, shipped together.

### Added — Package management (`/packages`)

- `internal/packages` auto-detects `apt-get` (Debian/Ubuntu) or `dnf`
  (Fedora/RHEL/Alma) at process start.
- Endpoints: `/api/packages/status` (public manager detection),
  `/search`, `/installed`, plus admin-only `/install`, `/remove`,
  `/upgrade`.
- Frontend `/packages` page with search + installed tabs and per-row
  install/remove action buttons.
- Strict name validator `^[a-z0-9][a-z0-9.+~_:-]*$` blocks anything that
  could escape argv. Run commands always pass `DEBIAN_FRONTEND=noninteractive`
  and `-y`.

### Added — WireGuard (`/wireguard`, admin-only)

- `internal/wireguard` wraps the `wg` CLI for status/dump parsing and
  for `wg set` peer add/remove. Generates Curve25519 keypairs in-process
  via `golang.org/x/crypto/curve25519` — no shelling out for crypto.
- Live status: per-peer endpoint, allowed-IPs, RX/TX byte counters and
  last-handshake age badges.
- "Add peer" dialog with optional Comment / Endpoint and Public-key
  field. Leave the key empty and the server generates a keypair, returns
  the private key exactly once for the user to paste into the client.
- Persists changes to `/etc/wireguard/<iface>.conf` so they survive
  reboot, and to the live interface via `wg set` so they apply immediately.
- 6 unit tests including key-generation uniqueness, CIDR-list validation,
  endpoint validation, dump parser, and peer-block removal in the config.

### Added — OIDC SSO

- `internal/auth/oidc.go` integrates `github.com/coreos/go-oidc/v3` for
  the standard code-flow with PKCE-equivalent state cookie.
- Config block:
    oidc:
      issuer: https://auth.example.com/realms/main
      client_id: webtermin
      client_secret: ...
      redirect_url: https://webtermin.example.com/api/auth/oidc/callback
      default_role: viewer
- Login page reads `/api/auth/oidc/status` and renders a "Sign in with
  SSO" button when configured. The button hits `/api/auth/oidc/start`,
  which 302s the browser to the IdP; the IdP returns to
  `/api/auth/oidc/callback` where we verify the ID token and issue a
  webtermin session cookie.
- First-time sign-ins create a local account keyed on the IdP's `sub`
  (with `preferred_username` or sanitised subject as the username).
  Existing accounts keep their already-stored role; new accounts get
  `oidc.default_role` (falls back to `viewer`).
- Audited as `auth.login.oidc` with `issuer=` detail. Local password
  login keeps working in parallel so the admin can recover if the IdP
  is unreachable.
- New top-level dependencies: `github.com/coreos/go-oidc/v3`,
  `golang.org/x/oauth2`, `github.com/go-jose/go-jose/v4` (transitive).

### Tests

- `packages.ValidName` accept/reject matrix.
- `wireguard`: ValidIface, GenerateKeypair uniqueness, validCIDRList,
  validEndpoint, parseDump on a 2-peer fixture, dropPeerBlock on a
  multi-peer conf, ClientConfig render snapshot.

## [0.7.0] — 2026-05-17

### Added — distribution

- `.rpm` and `.apk` packages alongside `.deb` — webtermin now installs
  cleanly on Fedora/RHEL/Alma and Alpine in addition to Debian/Ubuntu.
- Multi-arch container image published to `ghcr.io/satan1an/webtermin`
  (`latest` + `v<version>` tags, `linux/amd64` + `linux/arm64`).
- Release workflow logs into ghcr.io via the built-in `GITHUB_TOKEN` and
  uses `docker buildx` so cross-arch images come straight from the same
  CI run as the `.deb`/`.rpm`/`.apk` artefacts.

### Added — Backup module

- `/backup` page (admin-only) — create on-demand tar.gz snapshots of
  `/etc/webtermin`, `/var/lib/webtermin`, and any extra absolute paths
  you list. Download via signed `?` link, delete to free disk.
- Backups land in `${data_dir}/backups/` with names like
  `pre-upgrade-20260517-090530.tar.gz`. Restore is just
  `sudo tar xzf <file> -C /` from any shell.
- Backend (`internal/backup`) with path-safety validators (no traversal,
  no globs, no relative paths, no `/`). Sockets, devices, FIFOs are
  skipped — they're not meaningful on restore anyway.
- 5 unit tests covering the validators and a round-trip create-then-read.

### Added — non-root mode

- `packaging/sudoers.webtermin` template — drop into `/etc/sudoers.d/`,
  switch the systemd unit to `User=webtermin`, and the panel runs without
  root. Comments mark which capabilities are optional vs required.
- Comments in `packaging/webtermin.service` explain the trade-off and
  point at SECURITY.md for the full discussion.
- `.deb`/`.rpm`/`.apk` packages now ship the sudoers template at
  `/usr/share/doc/webtermin/sudoers.webtermin` so it's discoverable.

### Added — Docker image build

- `POST /api/docker/images/build` (operator) — multipart upload of a
  Dockerfile, builds it via the engine's `/build` endpoint, streams the
  layer-by-layer progress over WebSocket. Audited as
  `docker.image.build`.
- `internal/docker/build.go` adds `Client.Build(ctx, tarball, tag,
  dockerfile)`. Multi-file build contexts (with COPY of supporting
  files) are accepted by passing a pre-built tarball — the multipart
  endpoint currently wraps a single-file Dockerfile, the API supports
  the full case.

## [0.6.0] — 2026-05-17

### Added — Docker Compose stacks

webtermin can now deploy multi-container applications from a single
`docker-compose.yml`. Paste YAML, give the stack a name, click Deploy —
networks and volumes get created with `<stack>_<name>` prefixes,
containers come up in `depends_on` topological order, and everything is
labelled so the standard `docker compose` CLI keeps working too.

**Stacks page** (`/stacks`)
- List view with per-stack status (running / partial / stopped / empty),
  service count, container count, last-updated.
- Detail view shows the YAML in a Monaco editor (live syntax check, full
  themed editor) and groups live containers under their compose service.
- "Save & redeploy" button performs an idempotent redeploy — old
  containers are torn down, the new compose file becomes the source of
  truth, networks and volumes that already exist are reused.
- Start / Stop / Remove on the list view.
- Two-step confirm for Remove with explicit choice of "delete volumes
  (data gone)" vs "keep volumes" — losing a database volume by accident
  was the most-common Portainer complaint, so we made it harder.

### Added — backend

- `internal/compose` — yaml.v3 parser that supports both short-form and
  long-form for ports, volumes, env vars, labels, command and depends_on.
  `StringMapOrSeq` and `StringList` helpers absorb compose's "this field
  can be a map or a list" inconsistency.
- `compose.Manager` — orchestrates `docker.Client` to materialise stacks.
  Topo-sort honours `depends_on` (and detects cycles), declared networks
  & volumes are auto-created with stack-name prefix.
- New SQLite table `stacks(id, name, compose, created_at, updated_at)`
  with full CRUD in `store.stacks.go`. The compose YAML is persisted
  as-is — engine state stays authoritative for what's actually running.
- New endpoints under `/api/stacks` — list/get (viewer), create/update/
  start/stop/delete (operator). All mutations write audit-log entries
  with action prefix `stacks.*`.

### Added — tests

- `compose.Parse` round-trip on a representative multi-service stack
  including both map-form and list-form environment, short and long
  port specs, list-form command, depends_on ordering.
- `compose.ParsePort` / `compose.ParseVolume` validators against
  realistic inputs and metacharacter-laden adversarial ones.
- `compose.ToSpec` produces the right `CreateContainerSpec` (labels,
  network mode, restart policy, auto-start).
- `topoSort` honours depends_on, detects cycles, tolerates dangling deps.
- `ValidStackName` rejects upper-case, leading dash, too-long, etc.

### Notes

- Compose features deliberately out of scope for v0.6: build, healthcheck,
  deploy.resources, secrets, configs, profiles, extension fields.
- Multi-network attach for a single service is mapped to the first
  declared network (engine API quirk) — v0.7 follow-up will issue a
  follow-up `network connect` for the rest.
- Stack containers carry both `webtermin.stack` and standard
  `com.docker.compose.{project,service}` labels, so running
  `docker compose ls` on the host shows webtermin-deployed stacks too.

## [0.5.0] — 2026-05-17

### Added — Portainer-grade Docker management

The Docker page is now a full container-management workspace: containers,
images, networks, volumes, and engine system info — all reachable as tabs
without leaving the page.

**Containers**
- `Create container` form: image / name / restart policy / port bindings /
  env vars / bind+volume mounts / auto-start checkbox. Image is pulled
  automatically if not present on the host.
- `Remove container` with confirm and automatic `force=1` for running
  containers.
- `Live stats` dialog over WebSocket — CPU %, memory used/limit, network
  RX/TX, computed from the engine's `/containers/{id}/stats?stream=1`
  feed using the standard CPU-delta formula.
- `Console (exec)` — `docker exec -it <c> /bin/sh` right in the browser.
  Implemented as a hijacked HTTP/1.1 upgrade over the Unix socket (no
  HTTP-client abstraction is involved because Go's net/http doesn't
  expose hijack on response bodies), bridged to xterm.js over WebSocket.
  TTY resize events propagate to the container.

**Images**
- `Pull image` with live progress stream (engine's JSON event log over WS).
- `Remove image` with force flag.

**Networks** (new tab)
- List / inspect / create (driver, optional subnet) / remove.
- Built-in networks `host`, `bridge`, `none` are protected from
  accidental removal both in the UI and backend.

**Volumes** (new tab)
- List / create / remove (with force). Driver defaults to `local`.

**System** (new tab)
- Engine info card: version, storage driver, kernel, OS, arch, CPUs,
  memory, root dir.
- Disk-usage panel with per-kind size breakdown and admin-only `Prune`
  buttons for containers / images / volumes / networks.

### Added — backend

- `internal/docker` grew five files: `exec.go` (hijack-based PTY),
  `networks.go`, `volumes.go`, `images.go` (pull stream + remove),
  `system.go` (info / df / prune).
- New validators with unit tests: `ValidImageRef`, `ValidContainerName`,
  `ValidNetworkName`, `ValidVolumeName`, `ValidRestartPolicy`,
  `ValidPruneTarget`, plus the `splitImageRef` parser.
- New endpoints under `/api/docker`:
  - `POST /containers` create (operator)
  - `DELETE /containers/{id}` remove (operator)
  - `GET /containers/{id}/stats/stream` (viewer)
  - `GET /containers/{id}/exec/ws` (operator)
  - `GET /images/pull` over WS (operator)
  - `DELETE /images/{ref}` (operator)
  - `GET|POST|DELETE /networks` (read viewer, write operator)
  - `GET|POST|DELETE /volumes` (same)
  - `GET /info`, `GET /df` (viewer)
  - `POST /prune` (admin)

### Notes

- We deliberately did **not** pull in `github.com/docker/docker/client`
  even with this much new functionality — the hand-rolled HTTP-over-unix
  client now sits at ~600 LOC across the package. The official client
  would have been ~50 MB of transitive dependencies for what we use.
- Compose/stacks, image build from Dockerfile, registry auth, events
  stream, and volume content browser are intentionally deferred to v0.6+.

## [0.4.0] — 2026-05-17

### Added — three new modules

- **Cron** (`/cron`) — list, add and delete entries per Linux user via the
  standard `crontab -u` shadow utility. Schedule validator accepts the
  classic 5-field syntax (`0 3 * * *`) and documented aliases
  (`@reboot`, `@daily`, …); shell metacharacters in either schedule or
  command are rejected before exec. Backend writes the whole crontab at
  once over stdin, never assembling a shell string. Common-schedule
  presets in the UI.
- **Firewall** (`/firewall`, admin-only) — manages `ufw` rules:
  status (active/default in/out/forward/logging), rules list with
  numbered delete, add allow/deny rules, enable/disable the firewall.
  Spec validator allowlist: bare ports (`22`), port/proto pairs
  (`443/tcp`, `53/udp`), port ranges (`8000:8010/tcp`), named services
  (`ssh`, `http`), and `from <CIDR> [to any port N proto P]` clauses —
  anything outside this is rejected before reaching ufw. UI gracefully
  shows an "install ufw" prompt on hosts without it.
- **Docker** (`/docker`) — list containers and images, start / stop /
  restart / pause / unpause / kill containers, stream container logs
  over WebSocket with frame-header demultiplexing. Lifecycle actions are
  operator+; listings/inspect/logs are open to any authed user (it's the
  same surface area `docker ps` already gives).
  Implemented as a hand-rolled HTTP client over `/var/run/docker.sock`
  — no `github.com/docker/docker/client` dependency (which alone would
  have ~3× the binary size). Returns 503 with a clear message on hosts
  without Docker.

### Added — backend

- `internal/cron`, `internal/firewall`, `internal/docker` — three new
  packages with strict allowlist input validation and full unit tests
  for the validators / parsers.
- Server handler bundles: `handlers_cron.go`, `handlers_firewall.go`,
  `handlers_docker.go`. Every mutating action writes an audit-log entry.

### Added — tests

- `cron.ValidSchedule` / `ValidCommand` / `parseCrontab` / `splitEntry`
  (rejects shell substitution in the schedule field).
- `firewall.ValidSpec` / `parseRules` / `parseStatusHeader`.
- `docker.ValidContainerID` / `ValidAction` (rejects uppercase hex,
  oversized ids, names like `nginx`, traversal).

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

[Unreleased]: https://github.com/Satan1an/webtermin/compare/v0.9.1...HEAD
[0.9.1]: https://github.com/Satan1an/webtermin/releases/tag/v0.9.1
[0.9.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.9.0
[0.8.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.8.0
[0.7.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.7.0
[0.6.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.6.0
[0.5.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.5.0
[0.4.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.4.0
[0.3.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.3.0
[0.2.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.2.0
[0.1.0]: https://github.com/Satan1an/webtermin/releases/tag/v0.1.0
