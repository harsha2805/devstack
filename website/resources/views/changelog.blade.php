@extends('layout')

@section('title', 'Changelog · devstack')
@section('description', 'What changed in each version of devstack.')

@section('content')
<section class="band changelog">
  <div class="wrap">
    <header class="changelog-head">
      <h1>Changelog</h1>
      <p class="lede">What changed in each version of devstack.
        @if ($release) The newest is <a href="{{ $release['url'] }}">{{ $release['version'] }}</a>; installed copies offer it in Settings → Updates.@endif</p>
    </header>

    @forelse ($entries as $entry)
      <article class="release" id="v{{ $entry['version'] }}" aria-labelledby="v{{ $entry['version'] }}-title">
        <div class="release-meta">
          <a class="release-tag" id="v{{ $entry['version'] }}-title" href="#v{{ $entry['version'] }}">{{ $entry['version'] }}</a>
          @if ($entry['date'])
            <time datetime="{{ $entry['iso'] }}">{{ $entry['date'] }}</time>
          @else
            <span class="release-when">Unreleased</span>
          @endif
          @if ($entry['latest'])<span class="release-latest">Latest</span>@endif
        </div>
        <div class="release-body prose">
          @foreach ($entry['groups'] as $group)
            @if ($group['collapsed'])
              <details class="release-group">
                <summary><span class="release-group-title">{{ $group['title'] }}</span> <span class="release-count">{{ $group['count'] }}</span></summary>
                {!! $group['html'] !!}
              </details>
            @else
              @if ($group['title'] !== '')<h3>{{ $group['title'] }}</h3>@endif
              {!! $group['html'] !!}
            @endif
          @endforeach
        </div>
      </article>
    @empty
      <p class="fine">No releases yet.</p>
    @endforelse
  </div>
</section>
@endsection
