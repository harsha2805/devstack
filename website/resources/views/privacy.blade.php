@extends('layout')

@section('title', 'Privacy · devstack')
@section('description', 'What devstack sends over the network: an update check you can turn off, and the downloads you ask for.')

@section('content')
<section class="band">
  <div class="wrap narrow">
    <h1>Privacy</h1>
    <p class="lede">devstack has no telemetry, no account and no ads. Your sites, databases and settings stay on your
      computer. It connects to the internet only for the things below.</p>
    <div class="prose">
      <h2>The app</h2>
      <ul>
        <li><strong>Update check.</strong> Once a day, and 30 seconds after it starts, devstack reads
          <code>https://devstack.holostack.in/latest.json</code>. The request says only which version is asking
          (<code>devstack/0.1.0</code>); like any web request, the server sees your IP address. Turn it off in
          Settings → Updates, “Check for updates automatically”. The installer shows this before it installs.</li>
        <li><strong>Updates.</strong> When you click Update now, devstack downloads the new installer from
          github.com; GitHub's <a href="https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement">privacy statement</a> applies.</li>
        <li><strong>Servers.</strong> When you set up devstack or install a version, it downloads nginx from
          nginx.org, PHP from downloads.php.net and MySQL from cdn.mysql.com.</li>
        <li><strong>Links.</strong> Buttons such as Sponsor, the source code or Microsoft's Visual C++ runtime open
          in your browser only when you click them.</li>
      </ul>

      <h2>This website</h2>
      <p>No analytics and no tracking cookies. The site sets two technical cookies (a session and a security token)
        that expire after two hours. The web server keeps standard access logs (IP address, page, time, browser)
        for security and troubleshooting. The download count on the home page comes from GitHub's
        public release statistics.</p>

      <h2>Questions</h2>
      <p>Open an issue on <a href="{{ \App\Http\Controllers\SiteController::REPO }}/issues">GitHub</a>.</p>
    </div>
  </div>
</section>
@endsection
