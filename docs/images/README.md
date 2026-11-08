# Screenshots

UI captures used by the project README and Release notes.

| File             | Source page                | Notes |
| ---------------- | -------------------------- | ----- |
| `dashboard.png`  | `/dashboard`               | Hero shot — live CPU/mem/net charts, cropped to 1600×596. |
| `docker.png`     | `/docker`                  | Containers tab with per-container actions. |
| `terminal.png`   | `/terminal`                | Host-level web terminal. |
| `packages.png`   | `/network` → Install       | Dry-run plan dialog with the NetworkManager-over-networkd warning. |
| `firewall.png`   | `/firewall`                | ufw active with rules. |
| `stacks.png`     | `/stacks`                  | Compose stacks list. |
| `audit.png`      | `/audit`                   | Audit log, including a blocked package removal. |
| `cron.png`       | `/cron`                    | Per-user cron entries. |
| `users.png`      | `/users`                   | Linux system users (not panel users). |

To replace any of these:
1. Capture at 1600×900 in a dark-theme chromium window (no extensions, no devtools),
   against a throwaway host with demo data — never a real server.
2. Save here under the same filename.
3. Compress: `pngquant --quality=75-92 *.png` (or `oxipng -o 4 *.png`).
