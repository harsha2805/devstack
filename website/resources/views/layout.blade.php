<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>@yield('title', 'devstack: local nginx, PHP and MySQL for Windows')</title>
<meta name="description" content="@yield('description', 'devstack runs nginx, PHP and MySQL for every project folder on Windows, with per-site PHP versions, .test addresses, trusted HTTPS and AI guides for coding agents. Free and open source.')">
<link rel="icon" href="{{ asset('favicon.ico') }}">
<link rel="stylesheet" href="{{ asset('css/site.css') }}?v={{ filemtime(public_path('css/site.css')) }}">
<meta property="og:title" content="@yield('title', 'devstack')">
<meta property="og:description" content="Local nginx, PHP and MySQL for Windows. Free and open source.">
<meta property="og:image" content="{{ asset('img/dashboard-light.png') }}">
<meta property="og:url" content="{{ url()->current() }}">
</head>
<body>
<header class="top">
  <div class="wrap top-row">
    <a class="brand" href="{{ route('home') }}"><img src="{{ asset('img/icon.png') }}" alt="" width="28" height="28"> devstack</a>
    <nav class="nav" aria-label="Main">
      <a href="{{ route('home') }}#features">Features</a>
      <a href="{{ route('changelog') }}" @if(request()->routeIs('changelog')) aria-current="page" @endif>Changelog</a>
      <a href="{{ \App\Http\Controllers\SiteController::REPO }}">GitHub</a>
      <a class="nav-sponsor" href="{{ route('sponsor') }}">Sponsor</a>
    </nav>
  </div>
</header>

<main>
@yield('content')
</main>

<footer class="foot">
  <div class="wrap foot-row">
    <p>devstack is free and open source under the <a href="{{ \App\Http\Controllers\SiteController::REPO }}/blob/main/LICENSE">MIT license</a>.
      It downloads unmodified nginx, PHP and MySQL from their publishers.</p>
    <p><a href="{{ \App\Http\Controllers\SiteController::REPO }}">Source</a> · <a href="{{ route('changelog') }}">Changelog</a> ·
      <a href="{{ route('code-signing') }}">Code signing policy</a> · <a href="{{ route('privacy') }}">Privacy</a> ·
      <a href="{{ route('sponsor') }}">Sponsor</a> · <a href="https://holostack.in">HoloStack</a></p>
  </div>
</footer>
</body>
</html>
