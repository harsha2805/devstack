# devstack

A portable local web dev stack for Windows: Nginx/Apache, multiple PHP versions, MySQL, Redis, Mailpit,
automatic `*.test` sites with wildcard DNS and trusted local SSL. One exe, one config file.

Status: **core, downloads and desktop app working.** nginx + PHP (php-cgi pools, several versions side by side) +
MySQL under one supervisor, servers downloaded from their publishers and verified, sites served at
`http://<name>.localhost:<port>`, and a tray app with a dashboard. See [docs/SPEC.md](docs/SPEC.md) for the design and roadmap and
[docs/PLAN.md](docs/PLAN.md) for how it is built.

This is a clean-room project. It is designed from observable behavior and public documentation of the
underlying servers. No code, templates, or assets from other closed-source tools are used.

## Build

Requires Go 1.27+.

```powershell
.\build.ps1 -Version 0.1.0     # dist\devstack.exe (CLI) and dist\devstack-app.exe (tray + window)
go test ./...
```

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
devstack reload      # after adding a folder to www\ or editing stack.toml
devstack logs nginx -f
devstack db creds    # generated MySQL root password
devstack doctor
devstack down
```

`devstack up --foreground` runs in the terminal instead (Ctrl+C stops everything).

Servers are downloaded on demand from their publishers (windows.php.net, nginx.org, cdn.mysql.com) and checked
against the SHA-256 pins in `internal/packages/catalog.json`. Maintainers refresh that file with
`go run ./tools/catalog`. To reuse builds you already have, pass `init --bin D:\laragon\bin`. devstack runs
alongside other local stacks, and `init` moves to free ports when 80 and 3306 are taken.

## Layout

| Path | What |
|---|---|
| `stack.toml` | the only config you edit |
| `www\<site>\` | projects; `.devstack.toml` inside one overrides docroot, PHP version, aliases |
| `etc\` | generated config, rewritten on every render; don't edit |
| `etc-custom\nginx\*.conf` | your own nginx config, included and never touched |
| `data\` | MySQL data, daemon state, `secrets.toml` |
| `logs\` | `devstack.log`, `nginx-error.log`, `php-8.3-error.log`, `mysql.log`, `sites\*.access.log` |
