# Build plan

Date: 2026-10-26
Companion to [SPEC.md](SPEC.md). SPEC says *what* and *why*; this file says *how*.

- Part 1 (§0–§8): the core, M1. **Done.**
- Part 2 (§9): turning it into a public app: own root, server downloads, UI, installer.

## 0. Scope of "the basics"

M1 from the roadmap, plus the cheap parts of M2 that M1 needs to be useful:

- `stack.toml` loader with defaults and validation.
- Discovery of installed Nginx / PHP / MySQL builds in one or more `bin` folders.
- Site scan of `www/`: slugs, docroot detection, optional `.devstack.toml` per site.
- Renderer: `nginx.conf`, one server block per site, `php.ini` per PHP version, `my.ini`.
- Supervisor: start, health-check, respawn, graceful stop, log capture; Windows Job Objects so nothing is orphaned.
- Daemon with a token-protected HTTP API on `127.0.0.1`, plus a CLI that talks to it.
- Commands: `init`, `doctor`, `up`, `down`, `restart`, `reload`, `status`, `sites`, `logs`, `render`, `db creds`, `version`.

**Done when:** on this machine, with Laragon still running, `devstack up` serves `www/hello/index.php` at
`http://hello.localhost:8080/` through PHP 8.3, MySQL accepts the generated root password on its port, and `devstack down`
leaves no `nginx.exe`, `php-cgi.exe` or `mysqld.exe` of ours running.

Not in M1: watcher (M2), hosts-file writing (M2), DNS + SSL (M3), PATH shim (M4), Apache (M5), Redis/Mailpit/procs (M6),
UI (M7), downloads (M8). The model and renderer are shaped so these slot in without rewrites (see §7).

## 1. How the app looks to the user

### 1.1 First run

```text
> devstack init
Found:  nginx 1.30.4   D:\laragon\bin\nginx\nginx-1.30.4
        php   8.3.36   D:\laragon\bin\php\php8.3.36
        mysql 8.4.3    D:\laragon\bin\mysql\mysql-8.4.3-winx64
Ports:  http 80 is taken (httpd.exe pid 8256) -> using 8080
        mysql 3306 is taken (mysqld.exe pid 10164) -> using 3307
Wrote D:\devstack\stack.toml. Edit it, then run: devstack up

> devstack up
Rendering config ......... ok (etc\ rewritten, 3 sites)
mysql: first run, initializing data\mysql\8.4 ... ok (12s)
Starting nginx ........... ok  :8080
Starting php-8.3 x4 ...... ok  :9830-9833
Starting mysql ........... ok  :3307
mysql: root password set. See it with: devstack db creds

  hello   http://hello.localhost:8080   php 8.3   www\hello
  kbs     http://kbs.localhost:8080     php 8.3   www\kbs\public
```

### 1.2 Day to day

```text
> devstack status
devstack running  pid 14420  up 2h13m  api 127.0.0.1:52811

SERVICE   VERSION  STATE    PORT        PID     RESTARTS
nginx     1.30.4   running  8080        9912    0
php-8.3   8.3.36   running  9830-9833   4 procs 3
mysql     8.4.3    running  3307        10220   0

> devstack logs nginx -f        # tail logs\nginx-error.log
> devstack logs php-8.3 -n 100
> devstack reload               # rescan www, re-render, nginx -s reload (no downtime)
> devstack restart mysql
> devstack sites                # works without the daemon
> devstack doctor               # config, binaries, ports, folders; prints fixes
> devstack down
```

`*.localhost` resolves to 127.0.0.1 in Chrome, Edge, Firefox and curl without DNS or hosts edits, so M1 sites
are reachable on day one. `*.test` names are rendered too and start resolving once M2 (hosts) or M3 (DNS) lands.

### 1.3 Folder layout (root = folder holding `stack.toml`)

```text
D:\devstack\
  devstack.exe
  stack.toml            user-edited, the only source of truth
  www\<site>\           projects (or point `www` elsewhere, e.g. D:\laragon\www)
  bin\<svc>\<ver>\      server builds (M8 fills this; until then add more folders to paths.bin)
  etc\                  GENERATED, rewritten on every render
    nginx\nginx.conf
    nginx\sites\<slug>.conf
    nginx\default\index.html   landing page on http://localhost:8080 listing sites
    php\8.3\php.ini
    mysql\my.ini
  etc-custom\           user config, included but never touched (nginx\*.conf)
  data\mysql\8.4\       MySQL datadir (per major.minor)
  data\devstack\        daemon.json (pid, api addr, token), secrets.toml
  logs\                 nginx-error.log, php-8.3.log, mysql.log, sites\<slug>.access.log, devstack.log
  tmp\                  nginx prefix + temp dirs, php sessions/uploads
```

Note: SPEC put custom config at `etc/custom/`. It moves to `etc-custom/` so `etc/` can be wiped and regenerated whole.

### 1.4 `stack.toml`

```toml
www = "www"                 # relative paths resolve against the root
tld = "test"

[paths]
bin = ["bin", "D:/laragon/bin"]   # searched in order for <svc>/<anything>/

[services]
web   = "nginx"
mysql = "8.4"               # "" = newest installed, "8.4" = newest 8.4.x, "8.4.3" = exact

[php]
default = "8.3"
workers = 4                 # php-cgi processes per version
max_requests = 500          # PHP_FCGI_MAX_REQUESTS; process recycles, supervisor respawns
extensions = ["curl", "fileinfo", "gd", "intl", "mbstring", "mysqli", "openssl",
              "pdo_mysql", "pdo_sqlite", "sqlite3", "sodium", "zip", "exif"]
[php.ini]                   # free-form php.ini overrides
memory_limit = "512M"

[ports]
http  = 8080
mysql = 3307
php   = 9000                # base; version X.Y uses base + X*100 + Y*10 + worker (8.3 -> 9830..)
```

Per site, optional `www/kbs/.devstack.toml`:

```toml
docroot = "public"          # default: first of public, web, public_html, else the folder itself
php     = "8.3"             # honored in M1 if that version is installed; M4 adds the CLI shim
aliases = ["api"]           # api.kbs.localhost / api.kbs.test
```

## 2. Code layout

```text
cmd/devstack/main.go        flag parsing, dispatch, exit codes (0 ok, 1 error, 2 usage)
internal/config/            stack.toml + .devstack.toml types, defaults, validation, root discovery
internal/discover/          find nginx/php/mysql builds, probe versions, pick by constraint
internal/sites/             scan www, slugify, docroot detection, collisions
internal/model/             Resolve(config, discovered, sites) -> Stack (everything concrete: abs paths, ports)
internal/render/            embedded templates -> etc/, atomic writes, prune stale files
internal/supervisor/        Unit state machine, restart backoff, log capture
internal/services/          per-service specs: command line, readiness, init, graceful stop, reload
internal/daemon/            owns supervisor, HTTP API, daemon.json, single-instance lock
internal/client/            CLI side of the API
internal/winproc/           Job Objects, detached spawn, port-owner lookup (windows build tag)
internal/logfile/           size-capped rotating writer
```

Dependencies are kept small: `github.com/BurntSushi/toml`, `golang.org/x/sys/windows`, and
`github.com/go-sql-driver/mysql` (readiness ping, root password, `SHUTDOWN`, later `db create`). The CLI uses only
the standard library.

## 3. Data flow

```text
stack.toml ─┐
www/*/      ├─> config.Load ─> discover ─> sites.Scan ─> model.Resolve ─> render.Write(etc/)
bin dirs   ─┘                                               │
                                                            └─> services.Build ─> supervisor.Start
```

`model.Stack` is the only input to the renderer and the service builders. It is pure data (no I/O), so it is
easy to test and the same value drives `render`, `status`, and `sites`. Every render writes the full `etc/` tree
and deletes generated files that no longer belong. This is what fixes SPEC problem 1 (stale PHP paths).

## 4. Runtime design

### 4.1 Daemon lifecycle

- `devstack up` runs preflight (load, resolve, port check, render) in the CLI so errors show in the terminal. If no
  daemon is alive it spawns `devstack daemon` detached (`DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP`, with
  `CREATE_BREAKAWAY_FROM_JOB` when allowed so closing the terminal doesn't kill it), then polls `/v1/status` and
  prints progress until every unit is running or one fails.
- `devstack up --foreground` runs the daemon in the terminal. Ctrl+C means graceful shutdown. It is used for debugging and tests.
- The daemon takes an exclusive lock on `data/devstack/daemon.lock`, listens on `127.0.0.1:0`, and writes
  `daemon.json` `{pid, addr, token, started}`. Every request needs `Authorization: Bearer <token>`, so a web page
  in the browser can't drive the API.
- If `daemon.json` exists but the pid is dead, the file is stale. The CLI removes it and reports "stopped".

API (v1, JSON): `GET /v1/status`, `POST /v1/reload`, `POST /v1/restart?unit=`, `POST /v1/shutdown`.

### 4.2 Supervisor

Unit = one OS process. States: `stopped → starting → running → stopping → stopped`, plus `backoff` and `failed`.

- Start: spawn with `CREATE_NO_WINDOW`, stdout/stderr to the unit's log, assign to a per-unit Job Object
  (`KILL_ON_JOB_CLOSE`). The daemon holds the job handles, so if it dies Windows kills every child.
  `TerminateJobObject` kills a whole tree, such as nginx master + worker.
- Ready: the service's readiness probe (TCP connect, MySQL ping) must pass within a timeout, or the start fails.
- Respawn: on unexpected exit, restart with backoff (1s, 2s, 4s … 30s). More than 5 crashes in 60s → `failed`.
  An exit caused by php-cgi recycling after `max_requests` isn't counted as a crash.
- Stop: the service's graceful stop runs first, then the job is terminated after a timeout (nginx 10s, mysql 60s).
- Order: mysql and php start first, nginx last. Stop runs in reverse.

### 4.3 Services

| Service | Command | Ready | Graceful stop | Notes |
|---|---|---|---|---|
| nginx | `nginx.exe -p tmp/nginx/ -c etc/nginx/nginx.conf -e logs/nginx-error.log` | TCP :http | `nginx -s quit` (same `-p -c`) | `nginx -t` runs before start and before reload; reload = `-s reload` |
| php-X.Y #i | `php-cgi.exe -b 127.0.0.1:<port> -c etc/php/X.Y/php.ini` | TCP :port | kill (stateless) | env `PHP_FCGI_MAX_REQUESTS`, `PHP_FCGI_CHILDREN=0`; nginx `upstream phpXY` lists all workers |
| mysql | `mysqld.exe --defaults-file=etc/mysql/my.ini --console` | `PING` as root | SQL `SHUTDOWN` | empty datadir → `--initialize-insecure` first; then set the generated root password |

There is no php-fpm on Windows, and several php-cgi processes can't reliably share one port. Each worker therefore
gets its own port, and Nginx load-balances across them. If one worker is recycling, Nginx tries the next.

MySQL root password: a random password is generated and written to `data/devstack/secrets.toml` *before* the
`ALTER USER` runs, so a crash in between can be recovered (the next start tries the saved password, then none).
`mysqlx=OFF` and `bind-address=127.0.0.1` are set so nothing conflicts with Laragon's 33060.

### 4.4 Rendered config, key choices

- Nginx: `listen 127.0.0.1:<http>`. Front-controller `try_files … /index.php?$query_string` only when the docroot has
  `index.php`. `try_files $fastcgi_script_name =404` blocks the `cgi.fix_pathinfo` upload exploit. Dotfiles are
  denied except `.well-known`. `sendfile off`, so edited files are never served stale. `client_max_body_size 256m`.
  Our own `fastcgi` params file is written from the Nginx docs.
- `server_name <slug>.<tld> *.<slug>.<tld> <slug>.localhost *.<slug>.localhost` + aliases.
- PHP: an ini is generated per version. It has an absolute `extension_dir`, only enables extensions whose DLL exists
  (missing ones show up as a `doctor` warning), and points `error_log`, `session.save_path`, `upload_tmp_dir` and
  `sys_temp_dir` into `logs/` and `tmp/`. Opcache is on with `validate_timestamps=1`, `revalidate_freq=0`.
  `display_errors=On`, `E_ALL`.
- Default server on `localhost` serves the generated landing page that lists sites.
- Every generated file starts with `Generated by devstack. Do not edit; change stack.toml or etc-custom/.`

### 4.5 Ports and coexistence

`init` and `up` probe every port. On a conflict, the error names the owner (`port 80 is used by httpd.exe pid 8256`),
found via `GetExtendedTcpTable`. `init` picks free fallbacks (80→8080, 3306→3307). Everything binds 127.0.0.1 only.

## 5. Milestone tasks (M1)

1. Module, `main.go`, command table, `version`. 
2. `config`: types, defaults, root discovery (`--root` > `DEVSTACK_ROOT` > walk up from cwd > exe dir), validation errors with field names.
3. `discover`: scan bin roots, probe `nginx -v`, `php-cgi -v`, `mysqld --version`, pick by constraint.
4. `sites`: scan, slugify, docroot, overrides, collision suffixes.
5. `model.Resolve` and ports math.
6. `render` + templates + golden-file tests.
7. `winproc`: job objects, detached spawn, port owner.
8. `supervisor` + `services` (nginx, php-cgi, mysql incl. init and password).
9. `daemon` + `client`, then `up/down/status/restart/reload/logs`.
10. `init`, `doctor`, `sites`, `render`, `db creds`.
11. End-to-end check on this machine next to Laragon. Update README.

## 6. Testing

- Unit: config defaults and errors, slugify table, docroot detection on temp dirs, version constraint matching,
  port math, supervisor backoff (fake clock + fake process).
- Golden: `render` output for a fixed `model.Stack` compared against `testdata/golden/`, refreshed with `-update`.
- Integration (`go test -tags integration ./...`): real binaries from `DEVSTACK_TEST_BIN`; up → HTTP 200 from PHP →
  MySQL ping → down → no leftover processes.

## 7. How later milestones attach

- M2 watcher: **done.** The daemon rescans www every 1.5s (`internal/daemon/watch.go`) and calls the same
  `reload` path when the site list changes: a folder added, removed or renamed, a `.devstack.toml` edited, a
  `public/` or `index.php` appearing. A change must look the same on two scans in a row, so a folder being
  copied or cloned is picked up once it settles (measured: about 3s from new folder to served). Polling a scan
  instead of file notifications ignores churn inside projects (vendor/, node_modules/). stack.toml still takes an
  explicit reload, so a half-saved file is never applied. Still to do in M2: `*.test` names with the hosts-file
  fallback, asked for once (one admin prompt), because Windows itself doesn't resolve `*.localhost` for
  programs other than browsers and curl.
- M3: `dns` and `certs` packages. The renderer gains a `listen 443 ssl` block when `site.SSL`.
- M4: `model` already resolves a PHP version per site and runs one pool per version in use. The shim is a separate tiny exe.
- M5: a second renderer behind `services.web`. The model doesn't change.
- M6: `procs` become supervisor units with no readiness probe. Redis/Mailpit are just more services.
- M7: the UI talks to the same v1 API.
- M8: `packages` installs into `bin/`, and discovery already finds the result.

## 8. Decisions and open questions

Decided:
- Nginx also listens on `[::1]` when IPv6 loopback exists. Browsers resolve `*.localhost` to `::1` first, and a
  refused loopback connect on Windows stalls about 200ms before falling back to IPv4. Measured: 221ms → 3ms per request.
- Port checks read the TCP table instead of trying a bind, because binding `127.0.0.1:80` succeeds while another
  server holds `0.0.0.0:80` and then quietly takes its localhost traffic.
- A unit's first start failure is not retried. It is almost always config, and retrying hides the message.
  Respawn with backoff only applies after a unit has run.
- The CLI uses the stdlib, with no cobra. The command set is small, and this keeps the exe lean.
- php-cgi pool ports are `base + X*100 + Y*10 + i`, which replaces SPEC's 9081/9083 to leave room for 10 workers per version.
- Until M8, existing Laragon binaries are used via `paths.bin`. These are upstream open-source builds (nginx, PHP,
  MySQL), not Laragon code, so the clean-room rules are unaffected.

Open:
- Should `www` default to `D:\laragon\www` on this machine (instant use of existing projects), or start empty?
- When the daemon is running, should MySQL run even if no site uses it? (Default: yes; `services.mysql = "off"` disables it.)

---

## 9. Part 2: public app

Decided 2026-10-26: devstack ships as a **public download**, with a **tray icon + app window**, its **own document
root**, and a UI in the **visual language of Smart Awake** (HoloStack), borrowed rather than copied wholesale.

Order: D1 → D2 → D3 → D4. A public user has no Laragon, so downloads (D2) come before the UI is useful to anyone.

### D1. Install layout and root

- Per-user install, no admin (Inno Setup `PrivilegesRequired=lowest`, as Smart Awake does).
- App files go in `%LOCALAPPDATA%\Programs\devstack\`: `devstack.exe` (console CLI) and `devstack-app.exe` (GUI subsystem,
  tray + window). Two exes because a console exe flashes a terminal when started from the Start menu, and a GUI exe
  can't print to one.
- The stack root holds `stack.toml`, `www`, `bin`, `data`, `etc`, `logs`. It defaults to `C:\devstack`, falling back to
  `%USERPROFILE%\devstack` if C:\ isn't writable, and is chosen on first run. The path must be ASCII with no spaces:
  nginx, PHP extensions and MySQL on Windows have known trouble with other paths, and the check is cheap insurance.
- The chosen root is remembered in `%LOCALAPPDATA%\devstack\app.json`. `config.FindRoot` gains that as its last fallback,
  so the CLI works from any folder.
- The installer adds the app folder to the user PATH, so `devstack` works in any terminal.

### D2. Server packages (M8): done

Verified on a clean root with no Laragon: `devstack init` downloaded nginx 1.30.6, PHP 8.4.27 and MySQL 8.4.13
(304 MB, 40s), checked every sha256, and `up` served a PHP page with MySQL. `pkg install php 8.3` plus a site's
`php = "8.3"` ran 8.3 and 8.4 side by side after a reload. Found on the way: nginx refuses 8.3 short-name paths
(`C:\Users\ABCDEF~1\...`), so every root and bin path is now expanded to its long form.

| Server | Source | Integrity |
|---|---|---|
| PHP (NTS x64) | windows.php.net `releases.json` | sha256 published by php.net |
| nginx | nginx.org `nginx-X.Y.Z.zip` | nginx.org publishes PGP signatures, not hashes; the catalog tool records the sha256 |
| MySQL | cdn.mysql.com `mysql-X.Y.Z-winx64.zip` | sha256 computed by our catalog tool (MySQL publishes only MD5) |

- `catalog.json` is embedded in the exe: kind, version, url, sha256, size, top-level folder to strip.
  `go run ./tools/catalog` refreshes it at release time. Later, a signed remote catalog lets new PHP patch releases
  appear without a devstack release.
- Install: download to `bin/<kind>/.<version>.partial`, verify sha256, extract (rejecting `..` paths), and rename to
  `bin/<kind>/<version>/`. A failed or cancelled install leaves nothing behind. Discovery already finds the result.
- CLI: `devstack pkg list | install <kind> <version> | remove <kind> <version>`. `init` and the first-run screen
  install the defaults: latest stable nginx, PHP 8.4 (current) and MySQL 8.4 LTS.
- Downloaded on demand, never bundled. The installer stays small, and PHP (PHP License) and MySQL (GPLv2) come
  straight from their publishers. The UI links to their licenses.
- Prerequisite: PHP and MySQL need the Visual C++ 2015-2022 x64 runtime. `doctor` and the UI detect it and link to
  Microsoft's official installer (one UAC prompt, once).

### D3. UI (M7): first version done

Built and checked in the real app (WebView2 window, DPI-scaled, themed title bar), driven through the same API
the buttons use. First-run setup downloaded nginx and PHP, created the stack, and started it. Stop, Start, a second
launch (hands off to the running app) and Quit (stops servers, exits, nothing left running) all work. Found on the
way: setup must pick free PHP worker ports too, so it now moves `ports.php` to 10000+ when 98xx is taken.
Not yet: live theme switch while open (the title bar follows the theme at window open), and a native folder picker.

- `devstack-app.exe`: `fyne.io/systray` for the tray and `jchv/go-webview2` for the window. Both are pure Go on
  Windows (no cgo). Closing the window hides it to the tray. Quit stops the services. If the WebView2 runtime is
  missing, the dashboard opens in the default browser.
- The app process serves an embedded HTML/CSS/JS dashboard on `127.0.0.1` behind a random token. It calls the
  same packages as the CLI. `up`/`down` move from `cmd/devstack` into a shared `internal/control` package.
  No Node build step.
- Tray menu: state, Start/Stop all, Open devstack, Sites ▸ (open in browser), Open www folder, Quit.
- Window: status strip (RUNNING / STOPPED + one live line + services), Sites (open, folder, PHP), Logs,
  Settings (ports, PHP/MySQL versions = packages, start with Windows). First run: pick root → pick versions →
  download with progress → start.
- **Look, borrowed from Smart Awake's DESIGN.md:** warm neutral ramp, Terracotta accent for actions and selection only,
  green only for live state, Segoe UI Variable, flat cards with 1px hairlines and no shadows, light/dark from the
  system, `forced-colors` for high contrast, tabular numerals.
- **Not carried over for v1:** accent picker, theme-test project, animated settings flyout, narrow-width layout.
  These can be added once people use it.

### D4. Release: installer and update feed done

Decided 2026-10-26: the product is **devstack**, published by HoloStack, open source, hosted at
**devstack.holostack.in**. Steps: [RELEASING.md](RELEASING.md).

- `installer/devstack.iss`: Inno Setup, per user (`PrivilegesRequired=lowest`), `AppId=devstack`, installs both
  exes into `%LOCALAPPDATA%\Programs\devstack`, adds that to the user PATH, optional sign-in start (the same
  HKCU Run value as the app's setting). Before replacing or removing files it runs `devstack-app.exe --quit`,
  which closes the app, stops the servers and waits. Uninstall keeps the stack folder.
- `internal/update`: `latest.json` signed offline with Ed25519 (`tools/release`), public key pinned in the app.
  The installer's SHA-256 is trusted only once the signature is, and it's re-checked with the file locked just
  before launch. The same design as Smart Awake's, with Go's stdlib instead of ECDSA in .NET.
- In the app: a check 30s after launch and then daily (can be turned off), Settings → Updates with **Update now**.
  Updating stops the servers and sets `resume_servers`, so the new version starts them again. Portable copies
  open the download page.
- `build-release.ps1` builds into `dist\release\` (its own `bin\`, so a dev copy running from `dist\` doesn't
  block it): installer, portable zip, `SHA256SUMS.txt`, signed `latest.json`.

Verified on this machine with test builds in a scratch folder: a silent install, an upgrade over a running app,
an upgrade with servers running (app, daemon, nginx and PHP stopped, files replaced, 0.1.1 reopened), servers
resuming after an update, and an uninstall with servers running (all stopped, files, PATH and sign-in entry removed,
stack kept). Found on the way: when the parent's job forbids breakaway, the daemon spawn retried a used
`exec.Cmd` and failed with "exec: already started". It now retries with a copy.

Not yet tested: the in-app download from the real feed, since nothing is hosted yet.

### Open (owner decisions)

- Code signing. Open source makes Certum's open-source certificate (low cost) or SignPath Foundation (free, signs
  CI builds) the likely routes. Until then releases are unsigned and SmartScreen warns on first run.
- License for the repository (needed before it is published as open source).
