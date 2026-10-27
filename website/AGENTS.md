# devstack website

The site at https://devstack.holostack.in, a small Laravel app that lives in `website/` of the devstack repo.

- **No database, no Node build.** It runs on shared hosting (cPanel). Sessions and cache use files; CSS is a plain
  file in `public/css/site.css`; there is no Vite. Don't add anything that needs a database, a queue worker, or
  `npm run build` on the server.
- **Content comes from the repo.** `public/latest.json` (the app's signed update feed) and
  `resources/markdown/CHANGELOG.md` are copies made by `deploy.ps1`. Edit `../CHANGELOG.md` and run the release
  script instead of editing the copies. Never hand-edit `latest.json`: it is signed.
- **Downloads are on GitHub Releases.** The site links to the URL in `latest.json`; it never hosts installers.
- The download total comes from GitHub's API via `app/Support/Downloads.php`, cached an hour in the file cache;
  never call GitHub per request.
- Pages: `/` (home), `/changelog`; `/download` and `/sponsor` redirect. Logic is in
  `app/Http/Controllers/SiteController.php`, views in `resources/views/`.
- **On the server the app is split:** `domains/devstack.holostack.in/devstack/` (the app) and `public_html/`
  (Laravel's `public/`, with an `index.php` that loads `../devstack` and calls `usePublicPath`). Always read
  public files through `public_path()`, never `base_path('public/...')`.
- Test with `php artisan test`. Deploy with `.\deploy.ps1` (see `../docs/WEBSITE.md`).
- The look follows the devstack app: warm neutrals, one Terracotta accent, flat cards with hairlines, light and
  dark from the system.
