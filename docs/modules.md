# Modules

webtermin exposes 13 first-class modules. Every panel user has one of three
roles — `viewer`, `operator`, `admin` — and each module's actions are gated
by that role.

| Module        | Path              | Read role | Write role | Notes |
|---------------|-------------------|-----------|------------|-------|
| Dashboard     | `/dashboard`      | viewer    | —          | Live CPU / RAM / swap / network / disk via WebSocket (gopsutil + recharts). |
| Services      | `/services`       | viewer    | operator   | systemd units over D-Bus. Start / stop / restart / enable. Live `journalctl` tail. |
| Docker        | `/docker`         | viewer    | operator   | Containers (list, create, exec console, live stats), images (pull, build, remove), networks, volumes, system info, prune (admin). Hand-rolled `/var/run/docker.sock` client. |
| Stacks        | `/stacks`         | viewer    | operator   | Compose v3.x stacks: paste YAML, deploy, edit + redeploy, start/stop, remove with optional volume wipe. |
| Cron          | `/cron`           | viewer    | operator   | Per-user crontab editing via `crontab -u`. 5-field + `@aliases`. |
| Users         | `/users`          | viewer    | admin      | Linux system users (create, delete, password, SSH `authorized_keys`). |
| Files         | `/files`          | viewer    | operator   | Filesystem browser with Monaco editor. Strict `SafePath` enforcement. |
| Terminal      | `/terminal`       | operator  | operator   | In-browser PTY for the panel process owner (xterm.js + creack/pty). |
| API tokens    | `/api-tokens`     | viewer    | viewer     | Programmatic-access tokens scoped at issue time (`wt_<43-base64url>`). |
| Packages      | `/packages`       | viewer    | admin      | apt/dnf auto-detect. Search, install, remove, upgrade. |
| Firewall      | `/firewall`       | admin     | admin      | ufw rules (allow/deny by spec). Status + numbered rules + toggle. |
| Network       | `/network`        | admin     | admin      | NetworkManager interfaces. Static / DHCP / DNS / hostname. |
| WireGuard     | `/wireguard`      | admin     | admin      | Peers list, add (with in-process keypair gen), remove. Persists to `/etc/wireguard/<iface>.conf`. |
| Backup        | `/backup`         | operator  | admin      | Tar.gz snapshots of `/etc/webtermin` + `/var/lib/webtermin` + extra paths. |
| Panel users   | `/panel-users`    | admin     | admin      | Manage panel accounts: create, change role, reset password, remove. Last-admin guard. |
| Audit log     | `/audit`          | admin     | —          | Every mutating action, with user / IP / target / outcome. SQLite-backed. |

## Role hierarchy

Roles are hierarchical — a user with `operator` can do everything a `viewer`
can, plus the mutations listed in their column. `admin` is `operator` plus
the admin-only items.

* **viewer** — read-only: dashboard, service list, file browser, container
  list, Docker logs/stats, cron list, packages search, API tokens (own).
* **operator** — viewer + service start/stop/restart, file write/upload/delete,
  PTY terminal, Docker container actions + create + remove, image pull / build /
  remove, networks + volumes CRUD, cron entries, stack lifecycle, backup
  list/download.
* **admin** — operator + panel-user management, Linux-user mutations, audit
  log read, firewall, network, WireGuard, package install / remove / upgrade,
  Docker prune, backup create / delete.

## Audit-log action namespaces

Every mutating action lands in `audit_log` with an `action` field. Use these
prefixes when searching or aggregating:

| Namespace        | What it covers |
|------------------|----------------|
| `auth.*`         | login, logout, setup, 2FA, password change. `auth.login.oidc` for SSO. |
| `panel.user.*`   | create / delete / role-change / password-reset on panel accounts. |
| `panel.token.*`  | API token create / revoke. |
| `systemd.*`      | service start / stop / restart / reload / enable / disable. |
| `linux.user.*`   | Linux-user create / delete / password / ssh-key add+delete. |
| `files.*`        | write / mkdir / delete / chmod / upload / download. |
| `terminal.*`     | open / close. |
| `cron.*`         | add / delete. |
| `firewall.*`     | add / delete / toggle. |
| `network.*`      | static / dhcp / dns / hostname. |
| `wg.peer.*`      | add / remove. |
| `pkg.*`          | install / remove / upgrade. |
| `docker.*`       | container create / remove / action / exec.open+close, image.build+remove, network/volume CRUD, prune. |
| `stacks.*`       | deploy / update / start / stop / delete. |
| `backup.*`       | create / download / delete. |
