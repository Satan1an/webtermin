#!/usr/bin/env bash
# Run the e2e suite against a real webtermin booted with systemd in a
# privileged Debian container ("the stand").
#
#   e2e/run.sh [playwright args...]      e.g. e2e/run.sh -g "docker page"
#
# Env:
#   WT_BINARY   linux webtermin binary for the Docker host's arch, built with
#               the SPA embedded (default: ./webtermin; built if missing)
#   WT_KEEP=1   leave the stand running after the run (for debugging)
#
# Only the containers this script starts are removed on exit.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
image=webtermin-e2e
stand="wt-e2e-$$"
runner="$stand-pw"
pw_image=mcr.microsoft.com/playwright:v1.56.1-noble
fixture=e2e-hello

log() { printf '\033[1m[e2e]\033[0m %s\n' "$*"; }

# shellcheck disable=SC2329  # invoked via trap
cleanup() {
  local rc=$?
  docker rm -f "$runner" >/dev/null 2>&1 || true
  if [ "${WT_KEEP:-}" = 1 ]; then
    log "WT_KEEP=1: stand left running as $stand (remove with: docker rm -f -v $stand)"
  else
    # -v drops the stand's anonymous /var/lib/docker volume, nothing else.
    docker rm -f -v "$stand" >/dev/null 2>&1 || true
  fi
  rm -rf "${ctx:-}"
  exit "$rc"
}
trap cleanup EXIT INT TERM

# --- 1. binary --------------------------------------------------------------
bin=${WT_BINARY:-$root/webtermin}
if [ ! -x "$bin" ]; then
  log "no binary at $bin, building (SPA + Go)"
  (cd "$root/web" && npm ci --no-audit --no-fund && npm run build)
  if command -v go >/dev/null 2>&1; then
    (cd "$root" && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o "$bin" ./cmd/webtermin)
  else
    docker run --rm -v "$root":/src -v webtermin-gomod:/go/pkg/mod -w /src golang:1.26 \
      sh -c 'CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o webtermin ./cmd/webtermin'
    bin=$root/webtermin
  fi
fi

# --- 2. image (staged context: no need to send the whole repo) --------------
ctx=$(mktemp -d)
cp "$root/e2e/Dockerfile" "$bin" "$ctx/"
[ "$(basename "$bin")" = webtermin ] || mv "$ctx/$(basename "$bin")" "$ctx/webtermin"
cp "$root/packaging/webtermin.service" "$root/packaging/config.yaml" "$ctx/"
log "building image $image"
docker build -q -t "$image" "$ctx" >/dev/null

# --- 3. stand ---------------------------------------------------------------
log "starting stand $stand"
docker run -d --name "$stand" --hostname e2e \
  --privileged --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
  --tmpfs /run --tmpfs /run/lock \
  -v /var/lib/docker \
  "$image" >/dev/null

in_stand() { docker exec "$stand" "$@"; }

wait_for() { # wait_for <seconds> <description> <cmd...>
  local t=$1 what=$2; shift 2
  for _ in $(seq "$t"); do
    if "$@" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  log "timed out waiting for $what"
  in_stand journalctl -u webtermin -u docker --no-pager -n 80 || true
  return 1
}

wait_for 120 "webtermin on :8443" \
  in_stand curl -fsk https://127.0.0.1:8443/api/auth/status

token=$(in_stand journalctl -u webtermin --no-pager | grep -o 'setup_token=[^ "]*' | tail -1 | cut -d= -f2)
[ -n "$token" ] || { log "setup token not found in journal"; exit 1; }

# A container for the docker spec. Built from the panel's own static binary so
# the stand needs no registry access; it prints its version and exits.
wait_for 120 "dockerd in the stand" in_stand docker info
in_stand sh -c "tar -C /usr/bin -c webtermin | docker import -c 'CMD [\"/webtermin\",\"-version\"]' - e2e-local/hello:1 >/dev/null \
  && docker run --name $fixture e2e-local/hello:1"

# --- 4. specs ---------------------------------------------------------------
log "running Playwright"
rc=0
docker run --rm --name "$runner" \
  --network "container:$stand" --ipc=host \
  -e CI="${CI:-}" -e WT_SETUP_TOKEN="$token" -e WT_DOCKER_FIXTURE="$fixture" \
  -v "$root/e2e":/e2e -w /e2e \
  "$pw_image" \
  sh -c 'npm ci --no-audit --no-fund --loglevel=error && npx playwright test "$@"' sh "$@" || rc=$?

if [ "$rc" -ne 0 ]; then
  log "specs failed (rc=$rc); webtermin journal tail:"
  in_stand journalctl -u webtermin --no-pager -n 50 || true
fi
exit "$rc"
