@extends('layout')

@section('title', 'Code signing policy · devstack')
@section('description', 'How devstack releases are built and code-signed, and who approves them.')

@section('content')
<section class="band">
  <div class="wrap narrow">
    <h1>Code signing policy</h1>
    <p class="lede">Free code signing provided by <a href="https://signpath.io">SignPath.io</a>, certificate by
      <a href="https://signpath.org">SignPath Foundation</a>.</p>
    <div class="prose">
      <h2>What is signed</h2>
      <p>Every release's installer (<code>devstack_setup_&lt;version&gt;.exe</code>) and the two programs it installs,
        <code>devstack-app.exe</code> and <code>devstack.exe</code>, also found in the portable zip. Only files built from
        <a href="{{ \App\Http\Controllers\SiteController::REPO }}">devstack's source on GitHub</a> are signed. nginx, PHP and
        MySQL are never signed by devstack: they are downloaded from their publishers when you ask for them.</p>

      <h2>How a release is built</h2>
      <ul>
        <li>Each release is built and tested by GitHub Actions from a tagged commit, with the build scripts in the same
          repository (<code>.github/workflows/release.yml</code>). SignPath checks that the files it signs came from
          that build.</li>
        <li>Each signing request is approved by hand, by an approver listed below, before anything is signed.</li>
        <li>The update feed devstack reads is signed separately, with a key kept offline. The app refuses an update whose
          feed signature or installer checksum doesn't match.</li>
      </ul>

      <h2>Team</h2>
      <ul>
        <li><strong>Committers and reviewers:</strong> <a href="https://github.com/harsha2805">harsha2805</a></li>
        <li><strong>Approvers:</strong> <a href="https://github.com/harsha2805">harsha2805</a></li>
      </ul>
      <p>Changes from anyone else arrive as pull requests and are reviewed before they are merged.</p>

      <h2>Privacy</h2>
      <p>devstack sends nothing about you or your projects anywhere. It only checks once a day for a new version, which
        you can turn off, and downloads what you ask for. The <a href="{{ route('privacy') }}">privacy statement</a> lists
        every connection.</p>

      <h2>Reporting a problem</h2>
      <p>If you find a signed devstack file that misbehaves, or one that wasn't built from this repository, open an
        issue on <a href="{{ \App\Http\Controllers\SiteController::REPO }}/issues">GitHub</a>.</p>
    </div>
  </div>
</section>
@endsection
