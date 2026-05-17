# HTTP API reference

Every endpoint lives under `https://<host>:8443/api`. There are two ways to
authenticate:

* **Browser session** — POST `/api/auth/login` to get a cookie, then send the
  CSRF token from the response as `X-CSRF-Token` on every mutating request.
* **Bearer token** — issue one on `/api-tokens`, then send
  `Authorization: Bearer wt_<43-chars>` on every request. Token auth bypasses
  CSRF (no cookie to ride). Token role clamps the request, regardless of the
  owner's current role.

All responses are JSON. Streaming endpoints upgrade to WebSocket.

```bash
TOKEN=wt_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
curl -H "Authorization: Bearer $TOKEN" https://my-host:8443/api/system/info
```

## Auth

| Method | Path                              | Role     | Notes |
|--------|-----------------------------------|----------|-------|
| GET    | `/auth/status`                    | public   | Returns `{needs_setup, user?}`. The login page polls this on boot. |
| POST   | `/auth/setup`                     | public   | First-run admin creation. `{username, password}`. 409 if already configured. |
| POST   | `/auth/login`                     | public   | `{username, password, totp?}`. Sets session cookie. |
| POST   | `/auth/logout`                    | any      | Invalidates the session. |
| GET    | `/auth/me`                        | any      | Returns the current session info (user / role / csrf_token). |
| GET    | `/auth/audit`                     | admin    | Last 200 audit entries. |
| POST   | `/auth/2fa/enroll`                | any      | Generate a TOTP secret. Returns `{secret, otpauth_url}`. |
| POST   | `/auth/2fa/verify`                | any      | `{code}`. Activates the pending TOTP secret. |
| POST   | `/auth/2fa/disable`               | any      | Wipes the user's TOTP secret. |
| GET    | `/auth/oidc/status`               | public   | `{enabled, issuer}`. UI hides the SSO button when disabled. |
| GET    | `/auth/oidc/start`                | public   | 302 to the IdP. |
| GET    | `/auth/oidc/callback`             | public   | IdP redirects here; we mint a session. |

## System

| Method | Path                          | Role    | Notes |
|--------|-------------------------------|---------|-------|
| GET    | `/system/info`                | viewer  | One-shot hostname, OS, kernel, CPU model, RAM total, uptime. |
| GET    | `/system/metrics`             | viewer  | Single CPU / mem / disk / net snapshot. |
| GET    | `/system/metrics/stream`      | viewer  | WebSocket. One JSON sample per 2 s. |

## Services (systemd)

| Method | Path                                       | Role     | Notes |
|--------|--------------------------------------------|----------|-------|
| GET    | `/services?type=service`                   | viewer   | Units of a given type (defaults to `service`). |
| POST   | `/services/{name}/action`                  | operator | `{action: start\|stop\|restart\|reload\|enable\|disable}` |
| GET    | `/services/{name}/journal/stream`          | viewer   | WS `journalctl -u <name> -f`. |

## Cron

| Method | Path                            | Role     | Notes |
|--------|---------------------------------|----------|-------|
| GET    | `/cron/{user}`                  | viewer   | Returns parsed entries. |
| POST   | `/cron/{user}`                  | operator | `{schedule, command, comment?}` |
| DELETE | `/cron/{user}/{line}`           | operator | Removes the entry on line N (1-based). |

## Linux users

| Method | Path                                                   | Role     | Notes |
|--------|--------------------------------------------------------|----------|-------|
| GET    | `/linux/users?system=0`                                | operator | `system=1` includes UID < 1000. |
| POST   | `/linux/users`                                         | admin    | `{name, gecos?, shell?, home?, password?}` |
| DELETE | `/linux/users/{name}?remove_home=1`                    | admin    | |
| POST   | `/linux/users/{name}/password`                         | admin    | `{password}` |
| GET    | `/linux/users/{name}/keys`                             | operator | Lists `~/.ssh/authorized_keys`. |
| POST   | `/linux/users/{name}/keys`                             | admin    | `{key}` — single full authorized_keys line. |
| DELETE | `/linux/users/{name}/keys/{fingerprint}`               | admin    | Fingerprint = `SHA256:…`. |

## Files

| Method | Path                          | Role     | Notes |
|--------|-------------------------------|----------|-------|
| GET    | `/files/list?path=/etc`       | viewer   | |
| GET    | `/files/read?path=/etc/hosts` | viewer   | Up to 5 MiB. |
| GET    | `/files/download?path=…`      | viewer   | Streaming. |
| POST   | `/files/write`                | operator | `{path, content}` |
| POST   | `/files/upload`               | operator | multipart `dir` + `file`. |
| POST   | `/files/mkdir`                | operator | `{path}` |
| DELETE | `/files/delete`               | operator | `{path, recursive?}` |
| POST   | `/files/chmod`                | operator | `{path, mode: "0644"}` (octal string) |

## Terminal

| Method | Path                          | Role     | Notes |
|--------|-------------------------------|----------|-------|
| GET    | `/terminal/ws?rows=24&cols=80`| operator | WebSocket PTY. |

## Panel users + tokens

| Method | Path                                       | Role  | Notes |
|--------|--------------------------------------------|-------|-------|
| GET    | `/panel/users`                             | admin | |
| POST   | `/panel/users`                             | admin | `{username, password, role}` |
| DELETE | `/panel/users/{id}`                        | admin | Last-admin guard refuses if it would leave 0 admins. |
| POST   | `/panel/users/{id}/role`                   | admin | `{role: viewer\|operator\|admin}` |
| POST   | `/panel/users/{id}/password`               | admin | `{password}` |
| GET    | `/panel/tokens`                            | any   | Admin sees all; everyone else sees only own. |
| POST   | `/panel/tokens`                            | any   | `{name, role, expires_in_days?}`. Role capped at issuer's role. Returns plaintext exactly once. |
| DELETE | `/panel/tokens/{id}`                       | any   | Owner can revoke own; admin can revoke any. |

## Docker

Read endpoints are open to any authed user; lifecycle is operator+.

| Method | Path                                                 | Role     | Notes |
|--------|------------------------------------------------------|----------|-------|
| GET    | `/docker/containers`                                 | viewer   | |
| GET    | `/docker/containers/{id}`                            | viewer   | Inspect. |
| POST   | `/docker/containers`                                 | operator | `CreateContainerSpec` body. |
| DELETE | `/docker/containers/{id}?force=1`                    | operator | |
| POST   | `/docker/containers/{id}/action`                     | operator | `{action: start\|stop\|restart\|pause\|unpause\|kill}` |
| GET    | `/docker/containers/{id}/logs/stream?tail=200`       | viewer   | WS. |
| GET    | `/docker/containers/{id}/stats/stream`               | viewer   | WS. One JSON / sec. |
| GET    | `/docker/containers/{id}/exec/ws?shell=/bin/sh`      | operator | WS. PTY in container. |
| GET    | `/docker/images`                                     | viewer   | |
| GET    | `/docker/images/pull`                                | operator | WS. First frame: `{image: "nginx:latest"}`. |
| POST   | `/docker/images/build`                               | operator | multipart `dockerfile` + `tag?`. WS upgrade. |
| DELETE | `/docker/images/{ref}?force=1`                       | operator | |
| GET    | `/docker/networks`                                   | viewer   | |
| POST   | `/docker/networks`                                   | operator | `CreateNetworkSpec` |
| DELETE | `/docker/networks/{id}`                              | operator | |
| GET    | `/docker/volumes`                                    | viewer   | |
| POST   | `/docker/volumes`                                    | operator | `CreateVolumeSpec` |
| DELETE | `/docker/volumes/{name}?force=1`                     | operator | |
| GET    | `/docker/info`                                       | viewer   | engine version / driver / counts. |
| GET    | `/docker/df`                                         | viewer   | `docker system df` data. |
| POST   | `/docker/prune`                                      | admin    | `{target: containers\|images\|volumes\|networks}` |

## Stacks (Compose)

| Method | Path                          | Role     | Notes |
|--------|-------------------------------|----------|-------|
| GET    | `/stacks`                     | viewer   | List with live status. |
| GET    | `/stacks/{id}`                | viewer   | One stack with containers. |
| POST   | `/stacks`                     | operator | `{name, compose}` — deploys. |
| PUT    | `/stacks/{id}`                | operator | `{compose}` — replace + redeploy. |
| POST   | `/stacks/{id}/start`          | operator | |
| POST   | `/stacks/{id}/stop`           | operator | |
| DELETE | `/stacks/{id}?remove_data=1`  | operator | `remove_data=1` also drops the stack's named volumes. |

## Packages

| Method | Path                              | Role    | Notes |
|--------|-----------------------------------|---------|-------|
| GET    | `/packages/status`                | any     | `{manager: "apt"\|"dnf"\|""}` |
| GET    | `/packages/search?q=<query>`      | any     | |
| GET    | `/packages/installed`             | operator| |
| POST   | `/packages/install`               | admin   | `{names: [...]}` |
| POST   | `/packages/remove`                | admin   | |
| POST   | `/packages/upgrade`               | admin   | Empty `names` upgrades all. |

## Firewall (ufw)

| Method | Path                          | Role  | Notes |
|--------|-------------------------------|-------|-------|
| GET    | `/firewall/status`            | admin | Active + rules. |
| POST   | `/firewall/rules`             | admin | `{action: allow\|deny, spec}` — spec follows ufw allowlist regex. |
| DELETE | `/firewall/rules/{number}`    | admin | |
| POST   | `/firewall/toggle`            | admin | `{enabled}` |

## Network

| Method | Path                                          | Role  | Notes |
|--------|-----------------------------------------------|-------|-------|
| GET    | `/network/interfaces`                         | admin | Includes IP / DNS / method per device. |
| GET    | `/network/hostname`                           | admin | |
| POST   | `/network/hostname`                           | admin | `{hostname}` |
| POST   | `/network/interfaces/{name}/static`           | admin | `{address, gateway?, dns: []}` |
| POST   | `/network/interfaces/{name}/dhcp`             | admin | |
| POST   | `/network/interfaces/{name}/dns`              | admin | `{dns: []}` |

## WireGuard

| Method | Path                              | Role  | Notes |
|--------|-----------------------------------|-------|-------|
| GET    | `/wireguard/status?iface=wg0`     | admin | Server pubkey + per-peer stats. |
| POST   | `/wireguard/peers`                | admin | `{iface?, comment?, public_key?, allowed_ips, endpoint?}`. Empty `public_key` triggers server-side keygen; private key returned in response *once*. |
| POST   | `/wireguard/peers/remove`         | admin | `{iface?, public_key}` |

## Backup

| Method | Path                                      | Role     | Notes |
|--------|-------------------------------------------|----------|-------|
| GET    | `/backups`                                | operator | |
| POST   | `/backups`                                | admin    | `{name, paths?: []}`. Default paths: `/etc/webtermin` + `/var/lib/webtermin`. |
| GET    | `/backups/{id}/download`                  | operator | Streams the tar.gz. |
| DELETE | `/backups/{id}`                           | admin    | Deletes the file too. |

## Error shape

Non-2xx responses return `{"error": "human-readable message"}`. Common codes:

| Status | When |
|--------|------|
| 400    | Invalid input (validator rejected it). |
| 401    | Unauthenticated (no session / bad token). |
| 403    | Authenticated but lacks role, or bad CSRF token. |
| 404    | Resource not found. |
| 409    | Conflict — e.g. stack already exists. |
| 429    | Login lockout (includes `Retry-After`). |
| 503    | Backend tool unavailable (Docker not running, ufw not installed). |
