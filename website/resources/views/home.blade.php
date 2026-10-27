@extends('layout')

@section('content')
<section class="hero">
  <div class="wrap hero-grid">
    <div class="hero-copy">
      <p class="eyebrow">For Windows 10 and 11</p>
      <h1>Every project folder, served.</h1>
      <p class="lede">devstack runs nginx, PHP and MySQL for your projects. Drop a folder into <code>www</code> and it is live at
        <code>name.test</code> a few seconds later, on the PHP version it needs, over trusted HTTPS.</p>
      <div class="cta">
        @if ($release)
          <a class="btn primary" href="{{ $release['url'] }}">
            <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 2v8m0 0L4.5 6.5M8 10l3.5-3.5M3 13h10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Download devstack {{ $release['version'] }}
          </a>
        @else
          <a class="btn primary" href="{{ \App\Http\Controllers\SiteController::REPO }}">Coming soon: follow on GitHub</a>
        @endif
        <a class="btn" href="{{ \App\Http\Controllers\SiteController::REPO }}">View the source</a>
      </div>
      <p class="fine">Free and open source · Installs for your account, no administrator prompt · Windows 10/11, 64-bit
        @if ($release) · <a href="{{ route('changelog') }}">What's new in {{ $release['version'] }}</a>@endif
        · <a href="{{ route('code-signing') }}">Code signing policy</a></p>
      @if ($downloads)
        <p class="downloads" title="Installer and zip downloads from GitHub Releases, including in-app updates">
          <strong>{{ number_format($downloads) }}</strong> {{ $downloads === 1 ? 'download' : 'downloads' }} so far</p>
      @endif
    </div>
    <figure class="shot">
      <picture>
        <source srcset="{{ asset('img/dashboard-dark.png') }}" media="(prefers-color-scheme: dark)">
        <img src="{{ asset('img/dashboard-light.png') }}" width="1000" height="480"
          alt="The devstack dashboard: servers running, and four sites with their .test addresses, PHP versions and AI guide switches.">
      </picture>
    </figure>
  </div>
</section>

<section id="features" class="band">
  <div class="wrap">
    <h2>What it does</h2>
    <div class="grid">
      <article class="card">
        <h3>A folder is a site</h3>
        <p>Every folder in <code>www</code> gets an address, with no hosts file or config to edit. Laravel-style
          <code>public/</code> folders are found on their own. New folders go live within seconds; a clone still in
          progress is picked up once it is complete.</p>
      </article>
      <article class="card">
        <h3>The right PHP for each site</h3>
        <p>Run PHP 8.1 to 8.5 side by side and pick one per site from a dropdown. One project stays on 8.2 while the
          next one moves to 8.4.</p>
      </article>
      <article class="card">
        <h3><code>.test</code> works everywhere</h3>
        <p>Turn it on once and <code>shop.test</code> works in PHP, Postman and terminals, not only in browsers. Only
          <code>*.test</code> lookups go to devstack; real domains are never touched.</p>
      </article>
      <article class="card">
        <h3>HTTPS without warnings</h3>
        <p>Every site also answers at <code>https://</code> with a certificate Windows trusts. devstack's certificate
          authority can only sign local names, never real websites, and it is removed again when you turn HTTPS off.</p>
      </article>
      <article class="card">
        <h3>Servers from their publishers</h3>
        <p>nginx, PHP and MySQL download from nginx.org, windows.php.net and mysql.com, each checked against a pinned
          SHA-256. A failed or tampered download leaves nothing behind.</p>
      </article>
      <article class="card">
        <h3>Lives in the tray</h3>
        <p>Start and stop everything, open a site, read its logs or change its PHP from the dashboard. Signed updates
          install only when you click Update.</p>
      </article>
    </div>
  </div>
</section>

<section class="band alt">
  <div class="wrap split">
    <div>
      <p class="eyebrow">New</p>
      <h2>Your AI coding agent knows the setup</h2>
      <p>Coding agents guess how a project runs: the wrong <code>php</code>, a PHP with no extensions,
        <code>artisan serve</code> next to a site that is already served. Flip a site's <strong>AI guide</strong>
        switch and devstack writes down how this machine actually runs it.</p>
      <ul class="ticks">
        <li>Works with Claude Code, Codex, Cursor and Copilot through <code>AGENTS.md</code> and <code>CLAUDE.md</code>.</li>
        <li>Machine details stay out of git; your own text in those files is left alone.</li>
        <li>Rewritten whenever the PHP version, port or address changes. Passwords are never written down.</li>
      </ul>
    </div>
    <pre class="code" aria-label="Part of an AI guide"><span class="c"># .devstack/ai-guide.md (excerpt)</span>
## Site
- URL: https://shop.test
- The site is already being served by nginx 1.30.6.
  Don't start another server (php -S, artisan serve).

## PHP 8.3.36
$env:Path  = "C:\devstack\bin\php\8.3.36;" + $env:Path
$env:PHPRC = "C:\devstack\etc\php\8.3"
php -v; composer install

## Database (MySQL 8.4.13)
- Host 127.0.0.1, port 3306, user root
- Password: run `devstack db creds`</pre>
  </div>
</section>

<section class="band">
  <div class="wrap">
    <h2>Up and running in three steps</h2>
    <ol class="steps">
      <li><strong>Install.</strong> Run the installer. It sets devstack up for your account and adds it to your PATH.</li>
      <li><strong>Pick a folder and versions.</strong> devstack downloads nginx, PHP and MySQL (about 300 MB) and starts them.</li>
      <li><strong>Drop projects into <code>www</code>.</strong> Each one is a site. Open it from the dashboard or the tray.</li>
    </ol>
  </div>
</section>

<section class="band alt">
  <div class="wrap split">
    <div>
      <h2>Or from the terminal</h2>
      <p>Everything the app does is a <code>devstack</code> command, so scripts and coding agents can use it too.</p>
    </div>
    <pre class="code" aria-label="devstack commands"><span class="p">&gt;</span> devstack sites
SITE   URL                 PHP   DOCROOT
blog   https://blog.test   8.4   www\blog
shop   https://shop.test   8.3   www\shop\public

<span class="p">&gt;</span> devstack pkg install php 8.4
<span class="p">&gt;</span> devstack https on
<span class="p">&gt;</span> devstack logs shop -f</pre>
  </div>
</section>

<section class="band sponsor">
  <div class="wrap sponsor-row">
    <div>
      <h2>Free, and kept that way by sponsors</h2>
      <p>devstack has no paid tier and no telemetry. If it saves you time, sponsoring on GitHub keeps it going.</p>
    </div>
    <a class="btn primary" href="{{ route('sponsor') }}">
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 13.5S2.5 10.2 2.5 6.3A2.8 2.8 0 0 1 8 5a2.8 2.8 0 0 1 5.5 1.3c0 3.9-5.5 7.2-5.5 7.2z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>
      Sponsor on GitHub
    </a>
  </div>
</section>
@endsection
