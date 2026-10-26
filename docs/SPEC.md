# Clean-room local dev stack: survey + plan

Date: 2026-10-26
Source: read-only survey of `D:\laragon` (config files, templates, folder layout, hosts file, log).
`laragon.exe` was **not** opened, disassembled, or string-dumped (EULA §4 forbids it).

## Clean-room rules for this project

1. Spec comes from observable behavior and documented file formats only.
2. Don't copy Laragon's shipped scripts or templates verbatim. Write your own from the Apache/Nginx/PHP docs.
3. Don't reuse Laragon's name, icons, or language files.
4. Keep this spec as the record of where each design decision came from.

---

## 1. What Laragon does (observed)

| Area | How it works on disk |
|---|---|
| Layout | `bin/<service>/<version>/` per tool; `etc/` generated configs; `data/` DB data; `www/` projects; `usr/` user config |
| Active versions | `usr/profile/default.ini`: one `[service] Version=<folder>` per service |
| Preferences | `usr/laragon.ini`: autostart, run-at-startup, auto vhosts, doc root, which services to use |
| Auto virtual hosts | Each `www/<name>` → `etc/apache2/sites-enabled/auto.<name>.test.conf`; uses `<name>/public` if present; filled from `<<PORT>> <<HOSTNAME>> <<PROJECT_DIR>>` templates |
| Hostnames | Appends `127.0.0.1 <name>.test #laragon magic!` lines to `C:\Windows\System32\drivers\etc\hosts` (needs admin) |
| User-owned vhosts | Remove the `auto.` prefix and the tool stops regenerating that file |
| SSL | Local CA; per-site `<host>.crt/.key`; SSL variants of the vhost templates; port 443 |
| PHP in Apache | `etc/apache2/mod_php.conf` (mod_php, TS build), or `fcgid.conf` (php-cgi) |
| PHP in Nginx | `php_upstream` → php-cgi on `127.0.0.1:9001` (count set in laragon.ini) |
| Admin aliases | `/phpmyadmin`, `/adminer`, redis/memcached admin via `etc/apache2/alias/*.conf` |
| Package downloads | `usr/packages.conf`: `Name=URL` list of zips (PHP, Apache, Nginx, Node, MySQL, PostgreSQL, phpMyAdmin, DBeaver) |
| Quick app | `bin/laragon/sites.manifest.conf`: `Laravel=composer create-project ... %s`, WordPress tarball, Symfony; `AutoCreateDatabase=true` |
| Procfile | `usr/Procfile`: named commands, optional `autorun`, `env_file=`, `PWD=`, inline env vars |
| Terminal | Cmder + `bin/laragon/laragon.cmd`, which sets `PATH` to every active `bin/` tool; `usr/user.cmd` user hook |
| Extras | Mailpit, Redis, Memcached, ngrok (share a site), Cronical (cron on Windows), HeidiSQL, Notepad++, Git portable, 30+ UI languages |
| DB | MySQL `my.ini` generated from a template, `datadir` → `data/mysql-8.4`, root with empty password |

## 2. Problems found on this machine (what a better version should fix)

1. **CLI PHP ≠ web PHP.** Apache loads `php8.3.36` (`mod_php.conf`), but the terminal `PATH` (`laragon.cmd`) and `fcgid.conf` still point to `php-8.3.33-Win32-vs16-x64`. `laragon.log` shows repeated `File not found!: ...php-8.3.33...\php.ini-development`. Switching versions leaves stale references, so `php artisan` runs a different PHP than the browser.
2. **Wildcard aliases don't work.** Vhosts declare `ServerAlias *.<site>.test`, but the hosts file can't hold wildcards, so `api.kbs.test` never resolves.
3. **Hostnames with underscores and capitals** (`IKEA_BIX_CDP.test`, `Flipkart_Retail_API.test`). Underscores aren't valid in DNS hostnames and cause trouble with cookies, CORS, and some TLS clients.
4. **Every hosts-file edit needs admin.** Adding a folder triggers a UAC prompt.
5. **One PHP version for all sites.** No per-project PHP version.
6. **Weak Nginx SSL template**: `TLSv1 TLSv1.1` plus `RC4`/`EXP`/`SSLv3` ciphers. The Apache SSL config is fine (TLS 1.2/1.3).
7. **MySQL root password empty** by default.
8. **Config spread across 6 formats**: ini, profile ini, Procfile, packages.conf, sites.manifest, .cmd files.
9. **Leftovers**: two memcached versions, two PHP folders, old Redis 5 Windows port.

## 3. Product goals

- One small exe (tray app + CLI), portable, no admin after first setup.
- Every derived artifact is regenerated from one source of truth, so stale paths (problem 1) can't happen.
- Per-site settings: PHP version, docroot, SSL, aliases.
- Real wildcard `*.test` DNS.
- Secure defaults.

## 4. Architecture

```
devstack.exe (Go, single binary)
├── cli         devstack up | down | status | site ls | php use 8.4 | share kbs | logs
├── daemon      supervisor + config renderer + HTTP API on 127.0.0.1 (for the UI)
│   ├── config      load stack.toml + www/*/.devstack.toml → resolved model
│   ├── render      model → httpd.conf / nginx.conf / vhosts / my.ini / php.ini / env script
│   ├── supervisor  start/stop/restart, health checks, respawn, log capture (Windows Job Objects)
│   ├── dns         tiny resolver: *.test → 127.0.0.1 (127.0.0.1:53)
│   ├── certs       local CA + per-site wildcard certs (crypto/x509)
│   ├── packages    download/verify(sha256)/unzip into bin/<svc>/<ver>
│   └── watcher     www/ folder watcher → add/remove sites live
└── ui          tray icon + small window (Wails, later)
```

Why Go: single static exe, easy Windows process control, built-in TLS/x509, simple DNS libraries, cross-compiles. The UI can come later because the CLI and HTTP API cover everything.

### 4.1 Single source of truth

`stack.toml` (user-edited):

```toml
root = "D:/devstack"
www  = "D:/devstack/www"
tld  = "test"

[services]
web   = "nginx"        # or "apache"
mysql = "8.4"
redis = "7"
mail  = "mailpit"

[php]
default  = "8.3"
versions = ["8.1", "8.3", "8.4"]

[procs.queue]                # replaces Procfile
cmd     = "php artisan queue:work"
cwd     = "www/kbs"
autorun = true
env     = { QUEUE = "default" }
```

Optional per-site override `www/kbs/.devstack.toml`:

```toml
php     = "8.1"
docroot = "public"      # auto-detected if omitted
ssl     = true
aliases = ["api", "admin"]   # api.kbs.test, admin.kbs.test
```

Everything in `etc/` is **generated output**, fully rewritten on each change and marked "do not edit". Users who want a hand-written vhost put it in `etc/custom/`, which is included but never touched.

### 4.2 Hostnames and DNS

- Slugify folder names: `IKEA_BIX_CDP` → `ikea-bix-cdp.test` (lowercase, `_` → `-`).
- One-time admin step at install: add a Windows NRPT rule sending only `.test` queries to the local resolver:
  `Add-DnsClientNrptRule -Namespace ".test" -NameServers "127.0.0.1"`
- After that, new sites and wildcard subdomains need no admin and no hosts-file edits.
- Fallback mode: hosts-file entries between `# BEGIN devstack` / `# END devstack` markers, rewritten as one block.

### 4.3 Per-site PHP

- Run one `php-cgi.exe -b 127.0.0.1:<port>` pool per installed PHP version (8.1 → 9081, 8.3 → 9083, ...), N workers each. There is no PHP-FPM on Windows, so the supervisor respawns php-cgi processes and sets `PHP_FCGI_MAX_REQUESTS`.
- Nginx: per-site `fastcgi_pass 127.0.0.1:<port>`.
- Apache: `mod_proxy_fcgi` with `SetHandler "proxy:fcgi://127.0.0.1:<port>"` per vhost (NTS PHP builds; no mod_php).
- The terminal shim puts a small `php.exe` launcher first on `PATH`. It reads the nearest `.devstack.toml` (walking up from cwd) and runs the matching PHP version, so CLI PHP always equals web PHP.

### 4.4 SSL

- Generate the root CA once and install it into the **CurrentUser** Root store (no admin).
- Per-site cert with SANs `site.test` and `*.site.test`, ECDSA P-256, 825-day validity.
- TLS 1.2/1.3 only in both Apache and Nginx configs.

### 4.5 Security defaults

- All services bind `127.0.0.1` only. Opt-in LAN mode per site.
- MySQL root password generated at first run and shown once in `devstack db creds`.
- Admin tools (phpMyAdmin/Adminer) are local-only.
- Package downloads are verified against sha256 pins in the package catalog.

### 4.6 Packages

The catalog is `packages.toml` with `name, version, url, sha256, layout` per entry. Install unpacks into `bin/<svc>/<ver>/`. Switching versions is a config change followed by a re-render, which removes stale paths by design.

## 5. Roadmap

| Milestone | Scope | Done when |
|---|---|---|
| M1 core | stack.toml loader, Nginx + one PHP version + MySQL supervisor, `up/down/status` | `devstack up` serves `www/foo` at `localhost` |
| M2 sites | www watcher, slugified `*.test` vhosts, hosts-file fallback | new folder reachable as `foo.test` without restart |
| M3 DNS + SSL | local resolver + NRPT, local CA, wildcard certs | `https://api.foo.test` works with no browser warning |
| M4 multi-PHP | php-cgi pools per version, per-site `.devstack.toml`, PATH shim | site A on 8.1 and site B on 8.4 at the same time; CLI matches |
| M5 Apache | Apache renderer with mod_proxy_fcgi | `web = "apache"` passes the same tests |
| M6 extras | procs (Procfile replacement), Redis, Mailpit, quick-app (`devstack new laravel foo`), auto DB create | |
| M7 UI | Wails tray app over the daemon HTTP API | |
| M8 packages | catalog, download/verify, `devstack php install 8.4` | |

## 6. Migration from the current Laragon install

A `devstack import D:\laragon` command could:
- read `usr/profile/default.ini` + `usr/laragon.ini` → `stack.toml`
- point `www` at `D:\laragon\www` (or copy)
- reuse `data/mysql-8.4` once MySQL is stopped, with the same major version
- remove the `#laragon magic!` hosts lines after the new DNS works (with confirmation)
