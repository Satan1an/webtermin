# Contributing

PRs welcome. For anything non-trivial, open an issue first so we don't
both end up writing the same thing.

## Layout

```
cmd/webtermin/           main entrypoint
internal/
  auth/                  password hashing, sessions, TOTP, OIDC, RBAC role helpers
  audit/                 audit-log writer
  backup/                tar.gz snapshot creator
  compose/               docker-compose YAML parser + stack manager
  config/                config.yaml loader
  cron/                  per-user crontab CRUD
  docker/                hand-rolled /var/run/docker.sock client
  files/                 SafePath + read/write/upload/mkdir/chmod/delete
  firewall/              ufw wrapper
  network/               nmcli wrapper
  packages/              apt/dnf wrapper with distro auto-detect
  pty/                   spawns a TTY shell as a given user
  server/                HTTP/WS server, middleware, all route handlers
  store/                 SQLite-backed users, sessions, audit, tokens, stacks, backups
  store/storetest/       test helper: fresh ephemeral store per t.TempDir()
  system/                gopsutil-based metrics
  systemd/               D-Bus client (start/stop/restart, journal tail)
  users/                 Linux-user mgmt + SSH-key file ops
  wireguard/             `wg` wrapper + Curve25519 keypair gen
web/                     React + Vite + TS SPA, embedded into the Go binary via go:embed
packaging/               systemd unit, sudoers template, postinst/prerm/postrm
docs/                    these docs + screenshots
```

## Local build

Requires Go ≥ 1.25 and Node ≥ 22 (LTS).

```bash
make build           # produces ./webtermin for the host arch
make build-arm64     # cross-compiles for ARM64 (no extra toolchain needed)
make docker          # builds the local docker image
make docker-arm64    # multi-arch docker image via buildx
```

Run it:

```bash
cp config.example.yaml config.yaml
./webtermin -config config.yaml
# https://localhost:8443 → first-run setup
```

## Tests

```bash
go test -race -count=1 ./...
```

CI runs the same with `-race`. Add tests for any new validator or
parser before sending the PR — `internal/wireguard/wireguard_test.go`
is a good template for the "shell-out-but-validate-inputs-first"
pattern.

For HTTP-flow tests, see `internal/server/integration_test.go` and
`internal/server/rbac_test.go`. The `storetest` helper gives you a
fresh ephemeral SQLite per test under `t.TempDir()`.

## Linters

```bash
gofmt -l .              # must be empty
go vet ./...
staticcheck ./...        # `go install honnef.co/go/tools/cmd/staticcheck@latest`
gosec -severity high -confidence high -exclude=G104,G301,G302,G304,G703 ./...
govulncheck ./...
```

CI runs all of these on every push and PR. The gosec excludes are for
rules whose default checks are false-positives in a sysadmin tool that
intentionally reads/writes arbitrary admin-supplied paths — don't add
new excludes without a clear reason.

## Adding a new module

The established pattern looks roughly like this:

1. **`internal/<module>/<module>.go`** — wrap whichever CLI does the
   actual work. Validate every input with a regex *before* exec. Use
   `os/exec` with argv slices, never `sh -c`.
2. **`internal/<module>/<module>_test.go`** — unit tests for the
   validators and parsers. Aim for "rejects shell metacharacter" cases
   and at least one parser round-trip.
3. **`internal/server/handlers_<module>.go`** — REST handlers that
   delegate to the module. Use `s.auditXyz(...)` to log every mutation.
4. **`internal/server/routes.go`** — wire endpoints. Apply
   `s.protected(auth.RoleX, handler)` for the right RBAC role.
5. **`web/src/pages/<Module>Page.tsx`** — React page using the existing
   `@/components/ui` primitives + `api.get/post/del`. Stick to the
   visual language: 12-column grid, dark theme, lucide icons.
6. **`web/src/App.tsx`** + **`web/src/components/layout/AppLayout.tsx`** —
   add the route and the sidebar nav entry. Mark `adminOnly: true` if
   the module is admin-locked.
7. **`docs/modules.md`** + **`docs/api.md`** — append the rows.

## Style notes

* **Don't `exec.Command("sh", "-c", …)`.** Every shell call uses argv.
* **Don't trust the HTTP layer's validation.** The package next to the
  shell-out validates *again* — it's the last line before exec.
* **Audit every mutation.** Even if it's just a config tweak. The audit
  log is the most-used module by admins running a real deployment.
* **No comments that restate the code.** Use comments to explain
  *why* — a security rationale, a non-obvious workaround, the next
  expected refactor — not what the next statement does.

## Releases

Tags trigger GoReleaser via `.github/workflows/release.yml`. Bump
`CHANGELOG.md`, tag `vX.Y.Z`, push the tag. The workflow produces:

* `.deb` + `.rpm` + `.apk` × `amd64` + `arm64`
* `linux_amd64.tar.gz` + `linux_arm64.tar.gz`
* multi-arch Docker image at `ghcr.io/satan1an/webtermin:vX.Y.Z`
  + `latest`
* a GitHub Release with the CHANGELOG section embedded

If a release fails partway, fix and ship as `vX.Y.Z+1`. Don't rewrite
already-pushed tags — even if the previous release was a dud, leave it
as historical record.
