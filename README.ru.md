<div align="center">

<img src="docs/banner.svg" alt="webtermin — self-hosted панель управления сервером" width="100%">

<h3>Красивая, безопасная, self-hosted альтернатива Webmin для управления одним Linux-сервером.</h3>

[![CI](https://github.com/Satan1an/webtermin/actions/workflows/ci.yml/badge.svg)](https://github.com/Satan1an/webtermin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Satan1an/webtermin?display_name=tag&sort=semver&color=00DC82)](https://github.com/Satan1an/webtermin/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-22d3ee.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platforms](https://img.shields.io/badge/linux-amd64%20%7C%20arm64-555)](#установка)

🌐 **[English](README.md)** · **Русский**

</div>

---

**webtermin** — один статический Go-бинарник со встроенным React-фронтом. Из коробки работает только по HTTPS, проводит вас через создание администратора при первом запуске, использует пароли Argon2id, поддерживает TOTP 2FA, защищён CSRF-токенами и пишет полный аудит-лог. Каждое системное действие проходит через типизированный allowlist — **никаких shell-строк из пользовательского ввода, никогда**.

## Главное

| | |
|---|---|
| ⚡ **Один бинарник** | Статический Go-исполняемый файл (~13 МБ) с встроенной React-SPA через `go:embed`. Без runtime, без отдельного веб-сервера для статики. |
| 🔐 **Безопасность в основе** | TLS по умолчанию (self-signed автогенерация или ваши PEM) · Argon2id · HttpOnly/Secure/Strict cookies · CSRF-токены · строгий CSP · rate-limit + lockout · опциональная TOTP 2FA. |
| 📜 **Аудит-лог** | Каждое изменяющее действие записывается в SQLite — кто, что, когда, цель, результат, IP. Доступен в UI на странице `/audit`. |
| 🧩 **Модули** | Дашборд · systemd-сервисы · Linux-пользователи + SSH-ключи · файловый менеджер с редактором Monaco · веб-терминал. |
| 🎨 **Современный UI** | React + Tailwind + shadcn/Radix · recharts · Framer Motion · тёмная тема по умолчанию · адаптивный. |
| 🏗️ **ARM-ready** | Кросс-компиляция под `linux/amd64` и `linux/arm64` (OrangePi, Raspberry Pi 4/5, ARM cloud VMs). |

## Скриншоты

> Положите свои снимки UI в `docs/images/` — см. [docs/images/README.md](docs/images/README.md) за рекомендованными именами.

<table>
<tr>
<td width="50%"><img src="docs/images/dashboard.png" alt="Дашборд" /></td>
<td width="50%"><img src="docs/images/services.png" alt="Сервисы" /></td>
</tr>
<tr>
<td><img src="docs/images/terminal.png" alt="Веб-терминал" /></td>
<td><img src="docs/images/files.png" alt="Файловый менеджер с редактором" /></td>
</tr>
</table>

## Установка

### Debian / Ubuntu / OrangePi / Raspberry Pi (.deb)

Скачайте пакет под свою архитектуру из [последнего релиза](https://github.com/Satan1an/webtermin/releases/latest):

```bash
# Замените VERSION на номер версии (например, 0.1.0).
# ARCH определится автоматически: amd64 или arm64.
VERSION=0.1.0
ARCH=$(dpkg --print-architecture)
curl -fsSL -o webtermin.deb \
  "https://github.com/Satan1an/webtermin/releases/download/v${VERSION}/webtermin_${VERSION}_linux_${ARCH}.deb"
sudo apt install ./webtermin.deb
```

Пакет установит `webtermin` в `/usr/bin/`, положит systemd-юнит в `/lib/systemd/system/webtermin.service`, создаст `/var/lib/webtermin` для данных и `/etc/webtermin/config.yaml` как conffile (ваши правки сохраняются при обновлении) и запустит сервис.

Откройте **`https://<хост>:8443`** чтобы пройти первоначальную настройку.

### Docker

```bash
docker compose up -d --build
```

См. [`docker-compose.yml`](docker-compose.yml) — нужны bind-mount'ы хостовых директорий и `privileged: true`, потому что контейнер управляет хостом. Относитесь к паролю панели как к паролю root SSH.

## Сборка из исходников

```bash
make build         # нативный бинарник для текущего хоста
make build-arm64   # кросс-компиляция aarch64 (OrangePi / RPi)
make docker        # локальный Docker-образ
make docker-arm64  # arm64 Docker-образ через buildx
```

Требуется Go ≥ 1.25 и Node ≥ 22 (LTS).

## Конфигурация

Дефолтный конфиг лежит в `/etc/webtermin/config.yaml` (или `./config.yaml` при ручном запуске). Все параметры закомментированы в [`config.example.yaml`](config.example.yaml). Ключи, которые чаще всего хочется поменять:

| Ключ | По умолчанию | Комментарий |
| --- | --- | --- |
| `server.listen` | `0.0.0.0:8443` | Поставьте `127.0.0.1`, если впереди nginx/Caddy. |
| `server.tls_cert` / `tls_key` | _пусто_ | Пусто → self-signed cert генерируется в `data_dir/tls/`. Укажите путь к своим PEM для CA-выданного сертификата. |
| `data_dir` | `/var/lib/webtermin` | SQLite БД, сессии и автогенерированный TLS-cert. |
| `security.session_ttl_min` | `240` | Время жизни сессии в минутах. |
| `security.require_2fa` | `false` | Принудительно требовать TOTP 2FA для всех. |
| `security.max_login_attempts` | `5` | Сколько неудачных попыток с одного IP до lockout'а. |
| `terminal.default_shell` | _пусто_ | Fallback-shell для веб-терминала. |

После правки: `sudo systemctl restart webtermin`.

## Модель безопасности

- **Транспорт** — только HTTPS, TLS 1.2+, HSTS 2 года, X-Frame-Options DENY, строгий Content-Security-Policy.
- **Аутентификация** — Argon2id (64 МиБ, t=2, p=2). Session ID 256-битный, случайный, в HttpOnly + Secure + SameSite=Strict cookie. CSRF-токен обязателен на каждом изменяющем запросе. Опциональная TOTP 2FA на пользователя. Rate-limit + lockout при неудачных входах.
- **Системные действия** — каждое действие живёт за типизированным allowlist'ом. Имена юнитов сверяются с `^[A-Za-z0-9@_.\-:\\]+\.(service|socket|...)$`, имена пользователей — с `^[a-z_][a-z0-9_-]{0,31}$`, пути файлов должны быть абсолютными, очищенными, без `..`. **Никаких shell-строк, собранных из пользовательского ввода** — `useradd`, `chpasswd`, `journalctl` и т.п. вызываются через argv-слайс в `os/exec`.
- **Аудит** — каждое изменяющее действие пишется в `audit_log` (пользователь, IP, действие, цель, результат, детали). Доступен в UI на `/audit`.
- **Процесс** — работает от root (нужно для управления системными пользователями/сервисами/файлами). Hardening systemd-юнита: `ProtectKernelTunables=true`, `ProtectKernelModules=true`.

### Аудит

Перед тегированием v0.1.0 (16.05.2026) был проведён ручной review + `govulncheck`: аудит `exec.Command` argv, параметризации SQL, режимов файлов, атрибутов cookie; глубокий разбор auth, CSRF, WebSocket origin, безопасности путей и выполнения команд. Результаты и by-design трейд-оффы — в [SECURITY.ru.md](SECURITY.ru.md#пре-релизный-аудит-v010--2026-05-16).

Нашли уязвимость? См. [SECURITY.ru.md](SECURITY.ru.md).

## Когда что можно

| Сценарий | Можно? | Что нужно |
|---|---|---|
| Home-lab / NAS за роутером | ✅ да | Достаточно дефолтов |
| Сервер за Tailscale/WireGuard | ✅ да | Только клиенты с ключами могут даже увидеть :8443 |
| Локальный dev/test | ✅ да | `127.0.0.1` only |
| VPS с публичным IP | ⚠ осторожно | Reverse proxy + IP whitelist (или Cloudflare Access), 2FA обязательно, сложный пароль, fail2ban |
| Production с конфиденциальными данными | ❌ нет | Нет внешнего аудита, нет CVE-response процесса. Подождите v1.0 |
| Голый порт 8443 в интернет | ❌ нет | Даже с 2FA — слишком высокий риск 0day |

Подробнее в [SECURITY.ru.md](SECURITY.ru.md#где-можно-запускать-webtermin).

## Roadmap

- [ ] Тесты (unit + property/fuzz для `auth`, `files/SafePath`, парсинга SSH-ключей)
- [ ] RBAC — роли viewer/operator/admin (сейчас все пользователи панели = admin)
- [ ] API-токены с scope'ами — для автоматизаций
- [ ] Модуль cron / systemd timers
- [ ] Модуль firewall (ufw / nftables)
- [ ] Сетевые интерфейсы + WireGuard
- [ ] Управление пакетами (apt/dnf)
- [ ] Управление Docker-контейнерами
- [ ] Бэкапы `/etc` и `/var/lib/webtermin`
- [ ] Non-root режим запуска через sudo-allowlist
- [ ] Tarball + .rpm + .apk пакеты
- [ ] SAML/OIDC SSO

PR'ы приветствуются — заведите issue перед началом нетривиальной работы.

## Лицензия

[MIT](LICENSE) © 2026 Satan1an
