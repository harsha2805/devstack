<?php

use App\Http\Controllers\ReleaseHookController;
use App\Http\Controllers\SiteController;
use Illuminate\Support\Facades\Route;

Route::get('/', [SiteController::class, 'home'])->name('home');
Route::get('/changelog', [SiteController::class, 'changelog'])->name('changelog');
Route::get('/download', [SiteController::class, 'download'])->name('download');
Route::get('/download/portable', [SiteController::class, 'portable'])->name('download.portable');
Route::get('/sponsor', [SiteController::class, 'sponsor'])->name('sponsor');
Route::redirect('/code-signing', '/'); // linked from the 0.2.0 release notes
Route::view('/privacy', 'privacy')->name('privacy');
// /latest.json (the app's update feed) is a static file in public/, kept current by App\Support\Releases.
// The release workflow calls this after publishing (bearer RELEASE_HOOK_TOKEN; CSRF-exempt in bootstrap/app.php).
Route::post('/hooks/release', ReleaseHookController::class)->middleware('throttle:10,1')->name('hooks.release');
