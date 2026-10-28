# The website (devstack.holostack.in)

A small Laravel 13 app in `website/`: home page, changelog, `/download` and `/sponsor` redirects, and the app's update
feed at `/latest.json`. It has no database and no Node build, so it runs on ordinary shared hosting.

Releases reach the site without an upload. `app/Support/Releases.php` reads GitHub's release list: the newest
release's `latest.json` (copied to `public_html/latest.json` when it is newer, well formed and points at a GitHub
download; the app checks its signature), its `changelog.json` for `/changelog`, every release's files (the
portable zip and checksum links) and the download total (installers and zips; in-app updates count too). It asks
at most once a day, and at once when the release workflow calls `POST /hooks/release` with `RELEASE_HOOK_TOKEN`.
If GitHub can't be reached it keeps the last data in Laravel's file cache, and the copies `deploy.ps1` shipped
cover a fresh install. The README's badges show the download total from shields.io.

## Run it locally

```powershell
cd website
composer install
copy .env.example .env; php artisan key:generate
php artisan serve          # http://localhost:8000
php artisan test
```

Or let devstack serve it: link the folder into your stack's `www` (`mklink /J D:\devstack\www\devstack-web D:\devstack-development\website`)
and open `https://devstack-web.test`.

## Releases and deploys

A release is just `.\publish-release.ps1 -Version x.y.z` ([RELEASING.md](RELEASING.md)); the site updates itself.
Deploy (below) only when the website's own code or design changes. `deploy.ps1` still bundles the current
`latest.json` and changelog, so a fresh install has something to show before GitHub is first asked.

`RELEASE_HOOK_TOKEN` in `website\.env.production` must match the repository secret of the same name. Without it
the hook answers 404 and the site refreshes daily.

## Hosting layout

holostack.in's hosting (Apache, `host8.cloudindianserver.com`, IP `103.235.104.192`) gives each domain a fixed web
root, `domains/devstack.holostack.in/public_html`, which can't be pointed at Laravel's `public/`. So the app sits
next to it, outside the web root:

```
domains/devstack.holostack.in/
├── devstack/      the Laravel app: .env, app/, storage/, vendor/ ... (not reachable from the web)
└── public_html/   Laravel's public files, and an index.php that loads ../devstack
```

The `index.php` calls `$app->usePublicPath(__DIR__)`, so Laravel treats `public_html/` as its public folder; that's
where `latest.json` and the CSS are read from. `deploy.ps1` builds exactly this layout (`-Layout public_html`, the
default). For a host that can point the web root at `public/`, use `-Layout public` and extract the plain tree.

## One-time setup

1. **PHP 8.3 or newer** for the domain (Laravel 13 needs it), with `mbstring`, `openssl`, `fileinfo`, `tokenizer`,
   `ctype` and `xml` (on by default on most hosts).
2. **DNS:** holostack.in uses the hosting's nameservers (`ns11`/`ns12.dnshorizon.com`). In the panel's DNS zone for
   `holostack.in`, add an `A` record: name `devstack`, value `103.235.104.192` (or a `CNAME` `devstack` →
   `holostack.in`). Check with `nslookup devstack.holostack.in 8.8.8.8`.
3. **HTTPS:** once that name resolves, issue a Let's Encrypt certificate for `devstack.holostack.in` in the panel's
   SSL section. Until then the server answers with its own `host8.cloudindianserver.com` certificate, which
   browsers and the app reject. The app only accepts an `https://` feed.

## Each deploy

1. Run `website\deploy.ps1`. It writes `dist\website.zip` with `devstack/` and `public_html/` inside.
2. In the File Manager, open `domains/devstack.holostack.in/`, upload the zip there (next to `public_html`, not in
   it) and extract it, overwriting existing files.
3. Make sure `devstack/storage/` and `devstack/bootstrap/cache/` are writable (755 folders is usual), and that
   PHP can write `public_html/latest.json` (it replaces it when a release is published).
4. Delete the uploaded zip.
5. Check `https://devstack.holostack.in/` and `https://devstack.holostack.in/latest.json`.

Files removed from the site in a later version stay on the server until deleted by hand; extracting only adds and
overwrites.

With SSH or a panel terminal, run `php artisan config:cache && php artisan route:cache && php artisan view:cache` in
`devstack/` after extracting, and `php artisan optimize:clear` before the next upload.

`website/.env.production` holds the production `APP_KEY`. `deploy.ps1` creates it the first time; it is not in git,
so back it up with the release signing key.
