# Changelog

All notable changes to devstack are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/); versions use SemVer.
The release scripts need a section for the version they build and take the in-app update notes from the
bold lead-ins under it, so write each feature up here when it lands.

## [0.2.0] — unreleased

### Added
- **SQL Server for PHP.** Install `sqlsrv` and `pdo_sqlsrv` from Servers → Versions → PHP extensions, or with
  `devstack pkg install ext sqlsrv pdo_sqlsrv`. php.net's PHP builds leave them out, so devstack downloads the right
  build for each installed PHP version from php.net's PECL builds, checks it against a pinned SHA-256, adds it to
  `php.extensions` in `stack.toml` and restarts PHP. PHP versions installed later get it too. If Microsoft's ODBC
  Driver for SQL Server is missing (sqlsrv can't connect without it), the app says so and links to it. Stacks that
  copied in Microsoft's `php_sqlsrv_83_nts_x64.dll` by hand switch to the managed copy.

### Changed
- **Current PHP builds.** PHP 8.2 to 8.5 now download php.net's rebuilt zips (`8.4.27-1` and so on). The PHP
  versions are the same.
- **Signed releases.** Releases are now built by GitHub Actions from the tagged source and, once SignPath Foundation
  accepts the project, code-signed, so Windows can name the publisher instead of warning about an unknown app.
  The [code signing policy](https://devstack.holostack.in/code-signing) says how.
- **Publisher details.** `devstack.exe`, `devstack-app.exe` and the installer now carry the product name, version,
  publisher (HoloStack) and copyright in their file properties.
- **Privacy statement.** Setup now says, before installing, exactly what devstack connects to: the daily update
  check (which can be turned off) and the downloads you ask for. Also at
  [devstack.holostack.in/privacy](https://devstack.holostack.in/privacy).

## [0.1.0] — 2026-10-27

First release.

### Added
- **nginx, PHP and MySQL for every project folder.** Each folder in `www` is a site at `http://<name>.localhost`,
  with no hosts file or DNS setup. Laravel-style `public/` folders are found on their own, and projects kept
  elsewhere can be linked in (`mklink /J`). Several PHP versions run side by side, and each site can pick one.
- **Official server downloads.** nginx, PHP and MySQL come straight from nginx.org, windows.php.net and
  mysql.com, each checked against a pinned SHA-256. A failed or tampered download leaves nothing behind.
- **A tray app with a dashboard.** Sites (with Browse and Folder buttons), Servers (restart, database login,
  versions to install or remove), Logs and Settings, in light or dark to match Windows. Closing the window keeps
  devstack in the tray. Only one copy runs per Windows account; opening it again brings the window forward.
- **First-run setup.** Pick a folder and PHP and MySQL versions, and devstack downloads and starts them.
- **New folders go live on their own.** Adding, removing or renaming a folder in `www`, or editing its
  `.devstack.toml`, takes effect within about 3 seconds while the servers run. A folder still being copied or
  cloned is picked up once it is complete.
- **`.test` addresses for every program.** Optional: sites also answer at `<name>.test`, which works in PHP,
  Postman and terminals, not only browsers. Windows asks for administrator approval once; only `*.test` lookups
  go to devstack, and hosts-file entries still win.
- **Trusted HTTPS.** Optional: every site also answers at `https://` with a certificate Windows trusts. Windows
  asks once; no administrator rights. devstack's certificate authority can only sign local names, never real
  websites, and is removed again when HTTPS is turned off or devstack is uninstalled.
- **AI guide per project.** A switch on each site writes a guide for coding agents (Claude Code, Codex, Cursor,
  Copilot): the site URL, the exact PHP build and how to run it and Composer, the database, logs and devstack
  commands. Machine details stay out of git; `AGENTS.md` and `CLAUDE.md` get a short pointer, and your own text in
  them is left alone.
- **Updates.** devstack checks for a new version once a day (this can be turned off) and installs it only when you
  click Update now. Updates are signed; running servers stop for the update and start again afterwards.
- **Installer.** Installs for your account only, with no administrator prompt, and adds `devstack` to your PATH.
  Uninstalling keeps your stack folder with its sites and databases.
- **Sponsor link.** Settings → About links to GitHub Sponsors, for anyone who wants to support devstack.
- **Command line.** `devstack init`, `up`, `down`, `status`, `reload`, `restart`, `sites`, `logs`, `doctor`,
  `db creds`, `pkg`, `dns`, `https` and `guide`, for everything the app does.
