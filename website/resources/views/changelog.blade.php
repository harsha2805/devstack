@extends('layout')

@section('title', 'Changelog · devstack')
@section('description', 'What changed in each version of devstack.')

@section('content')
<section class="band">
  <div class="wrap narrow">
    <h1>Changelog</h1>
    <p class="lede">What changed in each version of devstack.
      @if ($release) The newest is <a href="{{ $release['url'] }}">{{ $release['version'] }}</a>.@endif</p>
    <div class="prose">
      {!! $html !!}
    </div>
  </div>
</section>
@endsection
