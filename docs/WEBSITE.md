# The website (devstack.holostack.in)

A small Laravel 13 app in `website/`: home page, changelog, `/download` and `/sponsor` redirects, and the app's update
feed at `/latest.json`. It has no database and no Node build, so it runs on ordinary shared hosting.

The home page shows how many times devstack was downloaded: the sum of GitHub's download counts for every
release's installer and zip (in-app updates fetch the same installer, so they count too). It asks GitHub's API at
most once an hour, keeps the last total in Laravel's file cache if GitHub can't be reached, and shows nothing until
the first download. The README's badges show the same total from shields.io.

## Run it locally

```powershell
cd website
composer install
copy .env.example .env; php artisan key:generate
php artisan serve          # http://localhost:8000
php artisan test
```

Or let devstack serve it: link the folder into your stack's `www` (`mklink /J C:\devstack\www\devstack-web D:\devstack\website`)
and open `https://devstack-web.test`.

## Release order

1. `.\build-release.ps1 -Version x.y.z` (needs the `## [x.y.z]` section in `CHANGELOG.md`).
2. `.\publish-release.ps1 -Version x.y.z`: the GitHub release with the installer, zip and checksums.
3. `website\deploy.ps1`: builds `dist\website.zip`, including `latest.json` only once step 2's installer URL is live.
4. Upload and extract the zip (below). Installed copies of devstack see the update within a day.

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
3. Make sure `devstack/storage/` and `devstack/bootstrap/cache/` are writable (755 folders is usual).
4. Delete the uploaded zip.
5. Check `https://devstack.holostack.in/` and `https://devstack.holostack.in/latest.json`.

Files removed from the site in a later version stay on the server until deleted by hand; extracting only adds and
overwrites.

With SSH or a panel terminal, run `php artisan config:cache && php artisan route:cache && php artisan view:cache` in
`devstack/` after extracting, and `php artisan optimize:clear` before the next upload.

`website/.env.production` holds the production `APP_KEY`. `deploy.ps1` creates it the first time; it is not in git,
so back it up with the release signing key.
