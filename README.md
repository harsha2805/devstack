# devstack

A portable local web dev stack for Windows: Nginx/Apache, multiple PHP versions, MySQL, Redis, Mailpit,
automatic `*.test` sites with wildcard DNS and trusted local SSL. One exe, one config file.

Status: **core, downloads and desktop app working.** nginx + PHP (php-cgi pools, several versions side by side) +
MySQL under one supervisor, servers downloaded from their publishers and verified, sites served at
`http://<name>.localhost:<port>`, and a tray app with a dashboard. See [docs/SPEC.md](docs/SPEC.md) for the design and roadmap and
[docs/PLAN.md](docs/PLAN.md) for how it is built.

This is a clean-room project. It is designed from observable behavior and public documentation of the
underlying servers. No code, templates, or assets from other closed-source tools are used.

## Install

Download `devstack_setup_<version>.exe` from [devstack.holostack.in](https://devstack.holostack.in). It installs
for your user only (no admin prompt) into `%LOCALAPPDATA%\Programs\devstack` and adds `devstack` to your PATH.
The app checks for updates once a day and installs one only when you click **Update now**
(Settings → Updates). Uninstalling keeps your stack folder with its sites and databases.

## Build

Requires Go 1.27+.

```powershell
.\build.ps1 -Version 0.1.0     # dist\devstack.exe (CLI) and dist\devstack-app.exe (tray + window)
go test ./...
```

Releases (installer, portable zip, signed update feed): see [docs/RELEASING.md](docs/RELEASING.md). What changed
in each version: [CHANGELOG.md](CHANGELOG.md).

## Desktop app

`devstack-app.exe` lives in the tray. Left-click opens the dashboard; closing the window keeps it running in the
tray; "Stop servers and quit" ends everything. The first launch asks for a folder (default `C:\devstack`) and
the PHP and MySQL versions, then downloads and starts them. Choosing a folder that already has a `stack.toml`
adopts that stack instead.

The dashboard has Sites (address, PHP version per site, folder), Servers (services, database login, versions to
install or remove), Logs and Settings (start servers with the app, open at sign-in). It is plain HTML/CSS/JS
served on 127.0.0.1 behind a per-launch token and shown in a WebView2 window; without WebView2 it opens in the
browser. `go run ./tools/uipreview` serves it alone for browser testing.

## Use

```powershell
devstack init        # downloads nginx/PHP/MySQL (sha256-verified), picks free ports, writes stack.toml + www\hello
devstack pkg list    # available and installed server versions
devstack pkg install php 8.3
devstack up          # renders etc\ and starts everything in the background
devstack status
devstack sites       # URLs, e.g. http://hello.localhost:8080
devstack reload      # after editing stack.toml (folders in www\ are picked up on their own)
devstack logs nginx -f
devstack db creds    # generated MySQL root password
devstack dns on      # <site>.test addresses for every program (one admin prompt; off / status)
devstack https on    # trusted HTTPS for every site (Windows asks once; off / status)
devstack guide add   # AI guide for coding agents in the current site's folder (also: remove, show)
devstack doctor
devstack down
```

`devstack up --foreground` runs in the terminal instead (Ctrl+C stops everything).

Servers are downloaded on demand from their publishers (windows.php.net, nginx.org, cdn.mysql.com) and checked
against the SHA-256 pins in `internal/packages/catalog.json`. Maintainers refresh that file with
`go run ./tools/catalog`. To reuse builds you already have, pass `init --bin D:\laragon\bin`. devstack runs
alongside other local stacks, and `init` moves to free ports when 80 and 3306 are taken.

## .test addresses

Sites always answer at `http://<name>.localhost`, which needs no setup but only works in browsers and curl:
Windows itself doesn't resolve `*.localhost`, so PHP, Postman and most CLI tools can't reach it. Turning on
**.test addresses** (Settings, the one-time question on the Sites tab, or `devstack dns on`) fixes that:

- Windows asks for administrator approval once, to add a Name Resolution Policy rule that sends only `*.test`
  lookups to `127.0.0.1`. Nothing else changes: real domains are never affected, and hosts-file entries
  (Laragon's, for example) still win.
- While the servers run, devstack answers those lookups itself, so every site and subdomain
  (`api.my-app.test`) works the moment its folder appears. There is no hosts file to edit per site.
- `stack.toml` records it as `services.dns = "on"`, and site URLs switch to `<name>.test`. Turning it off
  removes the rule again (one more prompt).

## HTTPS

Turn on **HTTPS** in Settings (or `devstack https on`) and every site also answers at `https://`, with a
certificate Windows trusts: Edge, Chrome, curl.exe and PowerShell show no warnings. Plain `http://` keeps working.

- devstack makes a certificate authority for your Windows account in `%LOCALAPPDATA%\devstack\ca`, and Windows asks
  you once to trust it. No administrator rights are needed.
- The CA can only sign local names: `*.localhost`, your TLD (`*.test`) and loopback addresses. Even a stolen CA
  key can't produce a certificate for a real website.
- Each site gets its own certificate (`name.test`, `*.name.test`, `name.localhost`, `*.name.localhost`), reissued
  when its names change and before it expires. New folders get one automatically.
- The landing page publishes the CA's empty revocation list, so strict Windows clients (curl.exe) can finish
  their revocation check.
- PHP, Node and Python keep their own trust lists. From code, call the `http://` address, or point the tool at
  `ca.crt`. The AI guide tells agents the same.
- Port 443 is used unless another program holds it; then devstack picks 8443 or another free port.
- Turning HTTPS off, or uninstalling devstack, removes the CA from Windows and deletes it.

## AI guide

AI coding agents don't know how a project runs on this machine, so they guess: the wrong `php`, a php.ini-less
PHP without extensions, `artisan serve` next to a site that is already served, database settings from nowhere.
The **AI guide** switch on each site (or `devstack guide add`) fixes that:

- `.devstack/ai-guide.md` gets this machine's details: site URL, the exact PHP build and how to run it and
  Composer with the stack's php.ini, the database host and port, log files, devstack commands, and what not to
  touch. The folder ignores itself in git, and devstack rewrites the guide on every start and reload, so it
  follows PHP version, port and path changes. Passwords are never written; it says to run `devstack db creds`.
- `AGENTS.md` (read by Codex, Cursor, Copilot and others) gets a short pointer to that file, and `CLAUDE.md`
  imports `AGENTS.md` for Claude Code. Both are safe to commit. devstack only edits between its
  `<!-- devstack:begin -->` markers, and turning the guide off removes exactly what it added.

## Layout

| Path | What |
|---|---|
| `stack.toml` | the only config you edit |
| `www\<site>\` | projects; `.devstack.toml` inside one overrides docroot, PHP version, aliases |
| `etc\` | generated config, rewritten on every render; don't edit |
| `etc-custom\nginx\*.conf` | your own nginx config, included and never touched |
| `data\` | MySQL data, daemon state, `secrets.toml` |
| `logs\` | `devstack.log`, `nginx-error.log`, `php-8.3-error.log`, `mysql.log`, `sites\*.access.log` |

## License

[MIT](LICENSE). devstack downloads nginx (BSD), PHP (PHP License) and MySQL (GPLv2) from their publishers at run
time; none of them are bundled.
