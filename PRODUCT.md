# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

The dashboard is plain HTML/CSS/JS (`internal/ui/web/`) served on 127.0.0.1 behind a per-launch token and shown in a
WebView2 window by the tray app (`cmd/devstack-app`); without WebView2 it opens in the browser. Window default
1000×700, minimum 720×540 (96-DPI pixels). The marketing site (`website/`, Laravel) is a separate surface.

## Users

Windows PHP developers, in four overlapping groups:

- **Laragon/XAMPP switchers** who already run a local stack and want per-site PHP, `.test` DNS and HTTPS without
  admin prompts or stale paths.
- **Agency developers** with client projects on different PHP versions, often kept outside `www`.
- **Developers working with AI coding agents** (Claude Code, Codex, Cursor, Copilot) who need agents to know how each
  site runs on this machine.
- **Newcomers** setting up their first local PHP + MySQL environment, who need every step and Windows prompt
  explained.

A typical user has **under 10 sites**.

## Product Purpose

devstack serves every project folder: drop a folder into `www` (or add one from anywhere) and it is live at
`name.test` seconds later, on the PHP version it needs, over trusted HTTPS, with MySQL beside it. Success is a
developer who never thinks about vhosts, hosts files, PATH or php.ini paths again, and whose terminal, browser and AI
agent all agree on how each site runs.

## Positioning

- One source of truth (`stack.toml`) regenerates every derived file, so the terminal's PHP can't drift from the
  site's PHP: `php` and `composer` follow the site you're in.
- Per-site PHP versions side by side, real wildcard `*.test` DNS, and a local CA limited to local names, with no
  administrator rights after one-time approvals.
- Servers downloaded from their publishers and checked against pinned SHA-256 hashes; signed updates; no telemetry.
- An AI guide per site that tells coding agents the URL, PHP build, database and logs.
- Clean-room: built from observed behavior and documented formats, not from Laragon's code (see `docs/SPEC.md`).

## Operating Context

The dashboard is used two ways, and both matter:

- **Glance and go:** open the window from the tray, check that the stack is running, open a site, its folder or a
  terminal, and close it again.
- **Hands-on control panel:** change a site's PHP version, turn extensions on, edit php.ini settings, install another
  PHP version, read logs while debugging, copy the database password.

Closing the window keeps devstack in the tray; "Stop servers and quit" ends everything. Everything the app does is
also available from the `devstack` CLI, and the README documents both. First run is a setup screen: pick a folder,
PHP and MySQL versions, then devstack downloads and starts them.

## Capabilities and Constraints

- Sites: each folder in `www` and each added project (`[projects]` in `stack.toml`) is a site with an address, a PHP
  version (default or pinned), an AI guide switch, a folder, and a terminal. A missing project folder is paused and
  listed, not dropped.
- Services: nginx, one php-cgi pool per installed PHP version, MySQL; each has a state, ports, restarts and a log.
- PHP per version: a php.ini settings form (grouped settings, extension switches, search, undo, locked values from
  `stack.toml`), open-as-text, and extensions devstack downloads itself (`sqlsrv`, `pdo_sqlsrv`, needing Microsoft's
  ODBC driver).
- Versions: PHP 8.1–8.5, MySQL and nginx builds to install or remove from a pinned catalog.
- Optional one-time setups: `.test` addresses (one admin prompt), trusted HTTPS (one Windows trust prompt), devstack's
  php first on PATH (admin prompt only when another PHP is on the system PATH).
- Settings: stack folder, `stack.toml`, start servers on open, open at sign-in, daily update check, update download
  with progress, about, sponsor.
- Constraints: no Node build step for the dashboard; plain files embedded in the Go binary. Polling API, not push.
  Windows 10 and 11 only.

## Brand Commitments

- Name is **devstack**, always lowercase. Publisher is HoloStack; author credit is in `NOTICE`.
- Voice, as in `CHANGELOG.md` and the app's copy: plain words, concrete outcomes, no unexplained jargon. Say what
  Windows will ask before it asks, and what a change does to files ("your files stay where they are").
- Never use Laragon's name, icons or language files in the product (clean-room rule, `docs/SPEC.md`).
- Free and open source under Apache 2.0; the only money ask is the GitHub Sponsors link.

## Evidence on Hand

- Dashboard screenshots: `website/public/img/dashboard-light.png`, `dashboard-dark.png`; app icon
  `website/public/img/icon.png`, `installer/devstack.ico`, tray icons in `cmd/devstack-app/icons/`.
- Feature truth: `README.md`, `CHANGELOG.md`, `docs/SPEC.md`, `docs/PLAN.md`.
- No testimonials, user counts, reviews or benchmarks exist. Don't invent them; the download badge is GitHub's live
  count.

## Product Principles

1. **Every folder, served.** The common path needs no configuration; anything optional is offered once, explained,
   and easy to skip.
2. **One truth, everywhere.** What the dashboard shows must match the browser, the terminal and the AI guide, because
   they're all generated from the same `stack.toml`.
3. **Explain before Windows asks.** Admin and trust prompts are named in advance, with what they change and what they
   never touch.
4. **Fast for the glance, complete for the fix.** The two-second check (running? open site) must stay instant, while
   deeper controls (php.ini, extensions, versions, logs) stay one clear step away, not on the same screen.
5. **Your files are yours.** devstack never deletes or rewrites user content without saying so; removing a site stops
   serving it, and edits keep comments.

## Accessibility & Inclusion

The dashboard follows the Windows light/dark setting, supports `forced-colors` (High Contrast) and
`prefers-reduced-motion`, and labels icon-only buttons. Keep all of these; keyboard focus must stay visible.
