# Screenshots

UI captures used by the project README and Release notes.

| File             | Source page                | Notes |
| ---------------- | -------------------------- | ----- |
| `dashboard.png`  | `/dashboard`               | Hero shot — live CPU/mem/net metrics. |
| `services.png`   | `/services`                | systemd unit list with action buttons. |
| `stacks.png`     | `/stacks`                  | Compose stacks list (empty-state shown here). |
| `cron.png`       | `/cron`                    | Per-user cron entries. |
| `users.png`      | `/users`                   | Linux system users (not panel users). |
| `terminal.png`   | `/terminal`                | Host-level web terminal. |
| `firewall.png`   | `/firewall`                | Graceful empty-state when `ufw` isn't installed. |

To replace any of these:
1. Capture at 1600×900 in a chromium incognito window (no extensions, no devtools).
2. Save here under the same filename.
3. Compress: `pngquant --quality=75-92 *.png` (or `oxipng -o 4 *.png`).
