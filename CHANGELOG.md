# Changelog

All notable changes to devstack are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/); versions use SemVer.
The release scripts need a section for the version they build and take the in-app update notes from the
bold lead-ins under it, so write each feature up here when it lands.

## [0.3.0] — 2026-10-30

### Added
- **PHP settings without editing a text file.** Servers > php.ini now opens a settings form for that PHP version
  instead of a 2,000-line file. The settings people change most are grouped and explained in plain words: showing
  errors, upload and form limits, memory and time limits, the time zone, OPcache and sessions. Every extension in
  that version's `ext` folder has an on/off switch with a line on what it's for, a search box finds any setting or
  extension, and "More settings" lists everything else your php.ini sets and lets you add your own. Values are
  checked as you type: sizes like `256M`, whole numbers, real time zones, and a warning when the form limit is
  smaller than the upload limit. Nothing is written until you press Save, which stays greyed out until something
  changes; closing with unsaved changes asks whether to save, discard or keep editing, and Undo puts any one setting
  back. Saving restarts that PHP version right away and says so; with the servers stopped, `php` in terminals uses
  the new settings at once and the servers use them when they start. A setting fixed by `[php.ini]` in `stack.toml`
  is shown locked with the value that wins. The form edits the same file in place, comments and all: Open as text,
  or the arrow button beside php.ini, opens it in your text editor, and if the file changes there while the form is
  open, Save stops rather than overwrite it.

## [0.2.8] — 2026-10-30

### Changed
- **The php.ini button opens the whole php.ini.** Each PHP version now has a full php.ini of its own, made from
  PHP's `php.ini-development` with devstack's settings, so every setting is there to change in place. Every
  extension in that version's `ext` folder is listed under "Dynamic Extensions": remove the `;` in front of
  `extension=gd` to turn it on, add one to turn it off, and save; that PHP version restarts with it within
  seconds. The app's extension switches edit the same lines. devstack still sets the paths into its own folders
  (`extension_dir`, `error_log`, temp folders, `session.save_path`), and `php.extensions` in `stack.toml` is now
  what a PHP version's php.ini starts with. A short php.ini from 0.2.7 is turned into a full one with your
  settings kept, and the old file is saved beside it as `php.ini.old`. Delete the file to start over.

## [0.2.7] — 2026-10-30

### Added
- **Projects outside www.** Sites > Add folder… serves a project from anywhere, such as `E:\clients\acme`, without
  moving it into `www`. Pick the folder and it's live under its own name; devstack only asks for a name when
  another site already has that one. A project gets everything a `www` site does: its own PHP
  version, HTTPS, the AI guide, and `php` in its terminals. The list lives in `[projects]` in `stack.toml`, so it's
  plain to see and edit. A project's name is kept for it, so a new folder in `www` can't take its address. If a
  project's folder goes missing (a drive unplugged, the folder moved), it's paused and listed in Sites rather
  than dropped, and it comes back by itself when the folder returns. devstack warns when a folder is on a network
  drive, inside WSL or synced by OneDrive, which all make PHP slow. Removing a project only stops serving it: your
  files stay where they are. From the command line: `devstack project add <folder> [--name N]`, `remove`, `list`.
- **php.ini in one click.** Each PHP version on the Servers tab has a php.ini button. It opens your own php.ini for
  that version (`etc-custom\php\8.3\php.ini`, created with examples the first time), which devstack adds to the end
  of the php.ini it generates, so your settings win and survive every reload. Save it and that PHP version restarts
  with them within seconds. Every site on that version gets them, and so do `php` and `composer` in its terminals.

### Changed
- **Per-site PHP settings apply in seconds.** A `.user.ini` in a site's docroot is the way to change settings like
  `memory_limit` or `upload_max_filesize` for one site, and edits to it now apply within 5 seconds instead of
  PHP's default 5 minutes. The README and each site's AI guide explain which file to use for one site, one PHP
  version or all of them.
- **Apache License 2.0.** devstack moves from the MIT license to the Apache License 2.0. It is still free and open
  source for any use; anyone who redistributes it, changed or not, keeps the `NOTICE` file crediting Harsha
  (HoloStack) and says what they changed. The installer and portable zip now include `LICENSE.txt` and
  `NOTICE.txt`. Releases up to 0.2.5 stay under MIT.

## [0.2.5] — 2026-10-28

### Changed
- **Releases published in one step.** Each release is now built, tested, signed and published on GitHub from its
  tagged source, and devstack.holostack.in follows within seconds, with links to the portable zip and the SHA-256
  checksums. The app itself is the same as 0.2.4.

## [0.2.4] — 2026-10-28

### Changed
- **Update size up front.** When a new version is available, Settings says how big the download is, on the card
  and on the Update now button. While it downloads, the bar shows how much is done out of the total. Steps with
  nothing to measure, like stopping the servers or starting the installer, show a moving bar so the card never
  looks stuck. A download server that doesn't send a size no longer shows a broken bar.

## [0.2.3] — 2026-10-28

### Added
- **php and composer per site in every terminal.** devstack now installs its own `php` and `composer` commands,
  first on PATH. They look at the folder you're in and run that site's PHP version with devstack's `php.ini`, so
  `php artisan optimize`, `composer install` and `composer dump-autoload` match what the site is served with, with
  no PATH switching. Outside every site they run the default PHP. `composer` runs a pinned, checksum-verified
  Composer, downloaded the first time it's used. Projects linked into `www` (`mklink /J`) work too, and
  `DEVSTACK_PHP=8.3` forces a version for one command.
- **Terminal button.** Each site in the sites list has a terminal icon that opens PowerShell in the site's folder,
  with its PHP ready. `devstack terminal <site>` does the same.
- **PATH check.** If another PHP comes first on PATH (Laragon, XAMPP, Composer-Setup), the sites list says so and
  **Fix** puts devstack's first; a PHP on the system PATH needs one administrator prompt. `devstack shims` and
  `devstack doctor` report it, and `devstack shims fix` fixes it.

## [0.2.2] — 2026-10-28

### Fixed
- **The right ODBC driver.** With ODBC Driver 17 and 18 both installed, PHP's SQL Server extensions connect through
  17, but devstack said 18. The app, `devstack pkg install ext` and the AI guide now name the driver PHP actually
  uses, and the guide lists every installed one. Driver 18 encrypts by default and 17 doesn't, so the guide also
  says to set encryption explicitly, so a project that connects here doesn't fail on a server with only 18.

### Changed
- **Simpler sites list.** Click a site's name to open its folder, and its address to open it in the browser. The
  Browse and Folder buttons are gone.
- **Placeholder warnings in the AI guide.** When `.env` still has template values such as `DB_HOST=REPLACE_ME` or
  `DB_PASSWORD=your_password`, the guide names those keys and tells the agent not to connect or run migrations
  until they're filled in. It never prints the values of secrets.

## [0.2.1] — 2026-10-28

### Fixed
- **Extensions per PHP version.** `php.extensions` is shared by every PHP version, but Microsoft's SQL Server DLLs
  carry one in their name (`php_sqlsrv_83_nts_x64.dll`). devstack now matches `sqlsrv` or `sqlsrv_83_nts_x64` to
  whichever build each PHP has (`php_sqlsrv.dll`, or `php_sqlsrv_85_nts_x64.dll` for PHP 8.5), so the same list works
  on every version. Before, PHP 8.5 quietly left both out.
- **Skipped extensions are logged.** An extension listed in `stack.toml` that a PHP version has no DLL for is now
  written to `devstack.log` as a warning, not only shown in the app.

### Changed
- **Sharper AI guide.** The guide in `.devstack/ai-guide.md` now lists extensions that were asked for but couldn't
  load, with the command that downloads them, and says the PHP build is NTS x64. Its Database section reads
  `DB_CONNECTION` from the project's `.env`: it names the host, says whether PHP loads the matching driver
  (`pdo_sqlsrv`, `pdo_pgsql`, `pdo_mysql`, `pdo_sqlite`) and, for SQL Server, whether Microsoft's ODBC driver is
  installed. A project whose database is elsewhere is no longer told to ask for MySQL. Guides refresh within 30
  seconds of a `.env` change.

## [0.2.0] — 2026-10-27

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
- **Releases built on GitHub.** Releases are now built and tested by GitHub Actions from the tagged source.
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
