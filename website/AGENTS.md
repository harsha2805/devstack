# devstack website

The site at https://devstack.holostack.in, a small Laravel app that lives in `website/` of the devstack repo.

- **No database, no Node build.** It runs on shared hosting (cPanel). Sessions and cache use files; CSS is a plain
  file in `public/css/site.css`; there is no Vite. Don't add anything that needs a database, a queue worker, or
  `npm run build` on the server.
- **Releases come from GitHub, not uploads.** `app/Support/Releases.php` reads GitHub's release list once a day
  (file cache), and at once when the release workflow calls `POST /hooks/release` (bearer `RELEASE_HOOK_TOKEN`).
  It copies the newest release's signed `latest.json` to `public/latest.json` and reads its `changelog.json`, the
  release files and the download total. Never call GitHub per request. `deploy.ps1` still ships copies of
  `latest.json` and `resources/markdown/CHANGELOG.md` as a starting point. Edit `../CHANGELOG.md` and release;
  never hand-edit `latest.json`: it is signed.
- **Downloads are on GitHub Releases.** The site links to the URLs GitHub lists; it never hosts installers.
- Pages: `/` (home), `/changelog`; `/download`, `/download/portable` and `/sponsor` redirect. Logic is in
  `app/Http/Controllers/SiteController.php` and `ReleaseHookController.php`, views in `resources/views/`.
- **On the server the app is split:** `domains/devstack.holostack.in/devstack/` (the app) and `public_html/`
  (Laravel's `public/`, with an `index.php` that loads `../devstack` and calls `usePublicPath`). Always read
  public files through `public_path()`, never `base_path('public/...')`.
- Test with `php artisan test`. Deploy with `.\deploy.ps1` (see `../docs/WEBSITE.md`).
- The look follows the devstack app: warm neutrals, one Terracotta accent, flat cards with hairlines, light and
  dark from the system.
