# Running webtermin without root

By default webtermin runs as `root` — that's the simplest way to manage
systemd services, create Linux users, write `/etc/*` configs, and so on.
For deployments that prefer least-privilege, the same binary can run as
an unprivileged `webtermin` system user that gets back specific
capabilities via `sudoers.d` + the docker group.

## Trade-offs

* **What you keep:** dashboard metrics, file manager (read), services
  read, audit log, panel-user management, API tokens, OIDC, terminal,
  backups (of paths your user can read).
* **What needs sudo:** systemctl, journalctl, useradd / userdel /
  chpasswd, ufw, crontab on other users, chmod / chown on host files,
  optionally apt-get / dnf.
* **What's awkward:** file manager *writes* outside the user's home
  directory go through a sudo'd `chmod`/`chown` chain — slower than
  direct ops as root. Same for cron edits on other users.

If you only really wanted to lock down "everything is root" — consider
this. If you want to disable individual modules (e.g. "no file
manager"), v0.9 doesn't support per-module disabling; v1.0+ might.

## Step-by-step

### 1. Create the system user

```bash
sudo useradd --system --home /var/lib/webtermin --shell /usr/sbin/nologin webtermin
sudo install -d -o webtermin -g webtermin -m 0700 /var/lib/webtermin
sudo chown -R webtermin:webtermin /etc/webtermin /var/lib/webtermin
```

### 2. Install the sudoers template

The `.deb`/`.rpm`/`.apk` package ships a sane default at
`/usr/share/doc/webtermin/sudoers.webtermin`. Move it into `/etc/sudoers.d/`
through `visudo` so syntax errors are caught:

```bash
sudo visudo -f /etc/sudoers.d/webtermin
```

Paste the contents of the template. Save and exit. `visudo` validates the
file before installing — if it complains, fix the syntax and try again.

The template grants:

* `systemctl`, `journalctl` — for the services module
* `useradd`, `userdel`, `usermod`, `chpasswd` — for the users module
* `ufw` — for the firewall module
* `chmod`, `chown` — for the file manager (comment out if you don't need it)
* `crontab` — for the cron module
* `apt-get` / `dnf` are commented out by default; uncomment if you want
  the packages module to work.

### 3. Give the user access to Docker (optional)

If you use the Docker module, the panel user needs to talk to
`/var/run/docker.sock`. Don't add it to sudoers — add it to the `docker`
group:

```bash
sudo usermod -aG docker webtermin
```

(This effectively gives the user root, since anyone in `docker` can
launch a container that mounts `/`. There's no way around that without
rootless Docker.)

### 4. Switch the systemd unit

The shipped unit defaults to `User=root`. Override it via `systemctl edit`:

```bash
sudo systemctl edit webtermin
```

In the editor that opens, type:

```ini
[Service]
User=webtermin
Group=webtermin
```

Save. systemd writes that to `/etc/systemd/system/webtermin.service.d/override.conf`
and merges it on top of the shipped unit.

### 5. Restart and check

```bash
sudo systemctl restart webtermin
ps -o user,pid,cmd -C webtermin    # should show "webtermin", not "root"
sudo journalctl -u webtermin -n 50 --no-pager
```

Open the panel and try every module you care about. The ones that need
sudo will work transparently (sudoers permits NOPASSWD on the exact
commands).

## Recovering if you locked yourself out

The sudoers template doesn't grant `passwd` on the panel user, so a
botched config can leave you unable to log into the panel. Two recovery
paths:

* **From the host shell** (SSH still works because we never touched its
  config) — edit `/var/lib/webtermin/webtermin.db` with `sqlite3` to
  reset a panel-user password hash, or just re-run first-run setup by
  removing the DB:

  ```bash
  sudo systemctl stop webtermin
  sudo rm /var/lib/webtermin/webtermin.db
  sudo systemctl start webtermin
  # open https://<host>:8443 — first-run wizard appears again
  ```

* **From physical / out-of-band access** if SSH is also broken — boot
  into single-user mode, mount the rootfs, edit `/etc/sudoers.d/webtermin`
  to comment out the problematic entry, reboot.

## Why isn't this the default?

Most webtermin deployments are home-lab and the user *is* root by
virtue of owning the box. Two extra config steps for nothing they care
about is friction.

If you have stronger threat models (multi-admin team, shared host,
compliance), the non-root mode is the right choice — and the modules
that genuinely need root will still work because of the sudoers
allowlist.
