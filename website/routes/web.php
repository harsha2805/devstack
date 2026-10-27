<?php

use App\Http\Controllers\SiteController;
use Illuminate\Support\Facades\Route;

Route::get('/', [SiteController::class, 'home'])->name('home');
Route::get('/changelog', [SiteController::class, 'changelog'])->name('changelog');
Route::get('/download', [SiteController::class, 'download'])->name('download');
Route::get('/sponsor', [SiteController::class, 'sponsor'])->name('sponsor');
// /latest.json (the app's update feed) is a static file in public/.
