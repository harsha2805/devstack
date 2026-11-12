# Changelog

All notable changes to devstack are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/); versions use SemVer.
The release scripts need a section for the version they build and take the in-app update notes from the
bold lead-ins under it, so write each feature up here when it lands.

## [1.4.2] — 2026-11-12

### Security
- **Your stack folder is yours alone.** A folder at a drive root, such as `C:\devstack`, let every account on the PC
  read the MySQL password and replace devstack's servers. Setup and the first start of 1.4.2 limit it to you, SYSTEM
  and administrators. `www` keeps its permissions in stacks made before 1.4.2, since it can hold a very large number of files.
- **A cloned project can't bring its own servers.** `php`, `composer` and `devstack` only use a `stack.toml` or
  shim map found above the current folder once that stack was set up or started on your account, or passed with
  `--root`. Before, a repository could ship one that named its own programs to run.
- **Fix PATH no longer puts a folder you can change on the system PATH.** When another PHP is on the system PATH,
  devstack's php and composer now go there as an administrators-only copy in `Program Files\devstack\shims`. A
  stack that used Fix PATH before shows the fix again, and uninstalling devstack removes the copy.
- **MySQL's root account never has an empty password.** A new data folder gets the saved password while it is
  created, rather than just after MySQL first starts.
- **The password prompt only takes your own Windows password**, not another account's.

### Fixed
- **Downloads no longer redraw the whole page several times a second.** Progress bars and numbers update in place,
  so buttons and text around them stay put.
- **Double-clicking Start, Install or Add folder runs it once**, without an error about a job already running.
- **The php.ini editor keeps its place.** Coming back to the window no longer scrolls it to the top, and closing it
  during a save or while it loads no longer shows stale settings.
- **The servers start faster.** PHP's workers start together instead of one by one.
- **nginx and PHP logs are rotated.** Site access logs, `nginx-error.log` and `php-X.Y-error.log` move to `.1`
  once they pass 10 MB, when the servers start.
- **Sites with long names that start alike get their own database user**, rather than sharing one cut to 32 characters.
- **php commands start faster**, and the app polls less: it reads `stack.toml` and scans the servers once per refresh,
  and a stopped Mailpit no longer slows every refresh.
- **High Contrast and screen readers**: switches and server marks stay visible in High Contrast, site states are
  announced, setup's version choices work with the arrow keys, and focus returns to Add site when its menu closes.

## [1.4.1] — 2026-11-10

### Security
- **A project's `.devstack.toml` can't add to nginx's config.** A `docroot` with quotes, `;`, `{`, `}`, `$` or `#`
  is refused, so a cloned project can't make nginx listen on the network or serve other folders.
- **Removing a version only removes a version.** `devstack pkg remove` and the app's Remove button accept the five
  known packages and a plain version number; a path in their place is refused instead of deleted.
- **A site can't take over MySQL's own accounts.** Create user refuses a site named `root` (and MySQL's other
  built-in users) and a `.env` whose `DB_DATABASE` is `mysql`, `sys`, `information_schema` or
  `performance_schema`.
- **Open in browser only opens local addresses.** An address written to look local, such as
  `http://a.test:x@elsewhere.example/`, is refused.
- **The dashboard can't run script it didn't ship with.** Its pages carry a Content-Security-Policy.
- **Paths with a `'` are written safely into phpMyAdmin's config.**
- **The release workflow reads the tag as text.** A tag name can no longer run as part of the build script.

### Fixed
- **A slow PHP request no longer holds up others.** nginx sends each request to an idle PHP worker (`least_conn`)
  instead of taking turns, so a long report, upload or Xdebug session doesn't make every fourth request wait.
- **The app could crash while downloads ran.** The dashboard read a job's progress while it was being updated, and
  the tray menu could be rebuilt from two places at once.
- **Switches go back when an action fails.** Declining a Windows prompt (`.test` addresses, HTTPS, PATH) or a failed
  PHP change left the switch or picker showing what you asked for, not what devstack has.
- **Setup and extension installs don't stall on "Lost contact".** The page stumbled on a job that had no steps yet.
- **Keyboard focus stays where it was** when part of the page redraws (opening a site, a switch, the password eye).
- **Follow off stops the log refreshing**; before, it made it refresh faster. Switching logs quickly no longer
  shows one file's text under another's name.
- **Saving php.ini no longer reports an error after a successful save** when no server needed a restart.
- **A stopped server shows one alert, not two.** The dark theme's error message is readable.
- **The MySQL password is forgotten after its minute** even when the app can't be reached at that moment.
- **Stopping waits for a reload in progress**, so a server started by that reload isn't left running and then killed.
- **A server whose child process outlives it is still noticed as stopped.**

### Changed
- **MySQL is set up for development speed.** New `my.ini` settings turn the binary log off and flush the InnoDB log
  once a second (`innodb_flush_log_at_trx_commit = 2`): migrations, seeders and tests commit much faster, and a
  power cut can lose the last second of writes.
- **Your own MySQL settings.** Any `.cnf` file in `etc-custom\mysql` is read after the generated `my.ini`, so
  its values win: put `[mysqld]` and `innodb_flush_log_at_trx_commit = 1` there to get the safe default back.
- **The dashboard rests while its window is hidden**, and the tray re-reads the site list every ten seconds
  instead of every two.

## [1.4.0] — 2026-11-10

### Added
- **Mailpit catches the mail your sites send.** Nothing reaches real inboxes while you develop: every email lands
  in Mailpit's inbox instead. New installs download it with the other servers; on an existing stack, install it on
  the new Mail page (10.7 MB, from Mailpit's GitHub releases, checked against a pinned SHA-256). It runs with the
  other servers on `127.0.0.1:1025` (SMTP) and `127.0.0.1:8025` (inbox), and accepts any login, so a site's
  `MAIL_USERNAME` and `MAIL_PASSWORD` can stay as they are. Caught mail is kept in `data\mailpit`.
- **A Mail page.** The sidebar shows how many messages are unread. The page lists the newest mail (click one to open
  it in Mailpit), the SMTP settings to copy, Open inbox, Mailpit's log, Restart and Turn off.
- **Send a site's mail to Mailpit.** A site's details say how its `.env` sends mail. When that isn't Mailpit, Send
  mail to Mailpit writes `MAIL_MAILER=smtp`, `MAIL_HOST=127.0.0.1` and `MAIL_PORT=1025` into it, and reminds you
  to run `php artisan config:clear` if Laravel has cached its config.
- **PHP's `mail()` goes to Mailpit too.** While Mailpit runs, devstack points every PHP version's `SMTP` and
  `smtp_port` at it, and gives `mail()` a sender (`devstack@localhost`) when your php.ini sets none.

### Changed
- **The AI guide knows about Mailpit.** It tells coding agents that mail is caught, whether the site's `.env` sends
  to Mailpit, and how to read caught mail through Mailpit's API to check what a site sent.
- **`stack.toml` has `services.mail`, `ports.smtp` and `ports.inbox`.** `mail = "off"` stops Mailpit; the default
  runs whichever version is installed. `devstack pkg install mailpit` and `devstack init --mail off` work from the
  command line.

## [1.3.1] — 2026-11-09

### Changed
- **The MySQL password locks again after one minute.** After Windows confirms it's you, the password now stays
  available for one minute instead of five.

## [1.3.0] — 2026-11-09

### Added
- **A MySQL user for each site.** When a site's `.env` points at devstack's MySQL and still signs in as root, its
  details offer Create a user for this site: a user named after the site, with a random password and access to
  only that site's database (created if it's missing). devstack writes the login into the `.env` as `DB_USERNAME`
  and `DB_PASSWORD`. New password gives that user a fresh one. If Laravel has cached its config, devstack reminds
  you to run `php artisan config:clear`.
- **Windows checks it's you before showing the MySQL password.** Show and Copy now ask Windows Hello (PIN, face or
  fingerprint), or your Windows password where Hello isn't set up, the way a browser does before showing a saved
  password. After that the password stays available for 5 minutes. It's on by default; turn it off under Settings →
  Privacy (which asks once more). Until you pass the check, the app doesn't send the password to the page at all.
- **An eye icon shows the password.** In a site's details and on the Database page, the eye shows or hides it and
  the copy icon copies it.

### Changed
- **The AI guide knows phpMyAdmin and site users.** It gives coding agents phpMyAdmin's address, opened on the
  site's database, says whether the site signs in as root or as a user of its own, and asks them to keep that user.
- **phpMyAdmin in the credits.** Settings → About and the README list phpMyAdmin (GPLv2) beside nginx, PHP and MySQL.
- **Narrower PHP pickers.** The PHP dropdown in the site list is only as wide as the version, and in a site's details
  the label and the dropdown share one line.

## [1.0.5] — 2026-11-09

### Added
- **phpMyAdmin opens your site's database.** A site's details now read its Laravel `.env` and say which database it
  uses. When that's devstack's MySQL, the button reads "Open chess in phpMyAdmin" and opens phpMyAdmin straight on
  that database. A `.env` that points at another server or port, or at SQLite, PostgreSQL or SQL Server, says so.
- **phpMyAdmin from a site's details.** The Database section always offers it: Open it when it's installed,
  Install phpMyAdmin when it isn't, or Turn on MySQL first when MySQL is off.

### Changed
- **The PHP picker shows only the version.** "8.3 ★" instead of "PHP 8.3 ★": the heading above it already says PHP.

## [1.0.4] — 2026-11-09

### Changed
- **A star marks the default PHP.** The PHP picker read "PHP 8.3 (default)" on every row that uses the default. It
  now shows "PHP 8.3 ★", and a "★ default" key sits beside the PHP heading in the site list and in a site's details.

### Fixed
- **Stacks can share one phpMyAdmin.** When two stacks used the same phpMyAdmin folder (through `paths.bin`), the
  last one to start pointed phpMyAdmin at its own MySQL for both. Each stack's nginx now tells phpMyAdmin which
  settings to use, so each one opens its own databases.

## [1.0.3] — 2026-11-09

### Added
- **phpMyAdmin.** Install it from the Database page (or `devstack pkg install phpmyadmin`) and manage your databases
  in the browser. It runs at `phpmyadmin.test` (or `phpmyadmin.localhost`) with the default PHP and opens already
  logged in as root, from Open phpMyAdmin on the MySQL card or the phpMyAdmin button in a site's details. It's only
  served while MySQL runs and only to this computer, and it never calls home. Downloaded from phpmyadmin.net and
  checked against a pinned SHA-256.
- **Turn MySQL on and off in the app.** A stack set up without MySQL used to say "MySQL is turned off in
  stack.toml" with no way out but editing the file. The Database page now has Turn on (with the newest installed
  version) and Turn off, and a site's details offer Turn on too.
- **Switch MySQL versions.** An installed MySQL version that isn't running has a Use button. Each version keeps its
  own databases, which stay on disk when you switch away.

### Changed
- **The PHP picker shows what a site runs.** Each installed PHP version is listed once, the default marked
  "(default)", and the version the site uses now is the one selected, in the site list and in a site's details.

### Fixed
- **MySQL's root password after switching versions.** A MySQL version used for the first time got an empty root
  password, because devstack thought the saved one was already set. It now sets the saved password on every new
  version, so the login on the Database page works for each.

## [1.0.2] — 2026-11-09

### Fixed
- **Your sites go where you installed devstack.** The installer asked for a folder, but the first launch still put
  the www folder, the servers and the databases in `C:\devstack`. The first-run screen now suggests the folder you
  picked in the installer, so choosing `D:\devstack` puts your sites in `D:\devstack\www`. New installs default to
  `C:\devstack`, and the installer turns down folders with spaces or non-English letters, which nginx, PHP and
  MySQL can't run from. Existing installs and stacks don't move.
- **A tidier PHP version picker.** The site panel listed the default version twice ("Default 8.3", then 8.3
  again) and grew one button wider with every PHP you installed, until the labels were cut off. It's now a dropdown,
  like the one in the sites list, and both list the default version once.

## [1.0.1] — 2026-11-05

### Fixed
- **Start works after a restart of Windows.** If Windows shut down while devstack was running, Start could fail the
  next day with "talk to daemon: … actively refused it". devstack had taken a new, unrelated program for its old
  background service, because Windows had reused that service's process ID. devstack now checks that its service
  really is the one running, and starts a fresh one when it isn't.

## [1.0.0] — 2026-11-04

### Changed
- **A new dashboard.** The devstack window is redesigned for 1.0. A graphite sidebar always shows whether your
  servers run, with Start or Stop, and gives every server its own slot: filled while it runs, an empty outline when
  it's stopped, hatched when it keeps crashing. Five pages replace the old tabs: Sites, PHP, Database, Logs and
  Settings. Servers used to hold the services, the database login and the downloads together; each now has a page
  of its own.
- **Site details beside the list.** Click a site to open its panel: its address with Open and Copy, its folder and
  a terminal, its PHP version, the AI guide, the database login, and links to its PHP error log and access log. The
  list keeps the PHP version and AI guide switch on every row, and a Find box narrows it down. Add site is one button
  with two choices: open the www folder, or add a folder from anywhere else.
- **Problems where you look.** When a server stops or keeps crashing, a banner at the top of every page names the
  sites it takes down, with the error, its log and a Restart button. A project whose folder went missing shows as a
  dashed slot until the folder is back. Setup steps that are still open (`.test` addresses, PHP in terminals) sit
  together in one "Finish setting up" card that goes away once they're done.
- **A page for each job.** PHP lists every installed version with its port, the sites that use it, its settings
  and extensions, its error log and Restart, then the versions you can install and the SQL Server extensions.
  Database shows the MySQL login with Show and Copy. Logs lists every log file, servers and sites apart, and follows
  the one you pick; every Log button in the app opens its file there. nginx and its versions moved to
  Settings → Stack.
- **Light, dark and small windows.** The new look follows Windows' light or dark setting, still works at the
  smallest window size (the sidebar folds down to icons), and keeps High Contrast and reduced-motion support.

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
