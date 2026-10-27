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

## One-time cPanel setup

1. **PHP version:** in *MultiPHP Manager* (or *Select PHP Version*), set the subdomain to **PHP 8.3 or newer**, with
   the `mbstring`, `openssl`, `fileinfo`, `tokenizer`, `ctype` and `xml` extensions (on by default on most hosts).
2. **Subdomain:** *Domains* → *Create a New Domain* (or *Subdomains*): `devstack.holostack.in`, with the document
   root set to **`devstack.holostack.in/public`**. Pointing it at `public/` keeps `.env`, `storage/` and the code
   out of reach of the web.
3. **HTTPS:** *SSL/TLS Status* → run AutoSSL for the subdomain. The app only accepts an `https://` feed.
4. **DNS:** if holostack.in's DNS isn't managed by this cPanel, add an `A` record for `devstack` pointing at the
   hosting server's IP.

## Each deploy

1. Run `website\deploy.ps1`. It writes `dist\website.zip`, with production dependencies and `.env` inside.
2. In cPanel *File Manager*, open `devstack.holostack.in/` (the folder above `public`), upload the zip and
   *Extract* it, replacing existing files.
3. Make sure `storage/` and `bootstrap/cache/` are writable (755 for folders is usual on cPanel).
4. Delete the uploaded zip.
5. Check `https://devstack.holostack.in/` and `https://devstack.holostack.in/latest.json`.

With SSH or cPanel's *Terminal*, `php artisan config:cache && php artisan route:cache && php artisan view:cache`
after extracting makes the site a little faster; run `php artisan optimize:clear` before the next upload.

`website/.env.production` holds the production `APP_KEY`. `deploy.ps1` creates it the first time; it is not in git,
so back it up with the release signing key.
