<?php

namespace App\Http\Controllers;

use App\Support\Releases;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\RateLimiter;

/**
 * POST /hooks/release: the release workflow (or publish-release.ps1) calls it
 * right after publishing, with RELEASE_HOOK_TOKEN as a bearer token, so the
 * site serves the new feed and changelog at once instead of on its daily
 * refresh. It answers with the version public/latest.json now holds.
 *
 * Limits, on top of the route's 10 calls a minute per IP: 5 wrong tokens lock
 * an IP out for 10 minutes, and even valid calls ask GitHub at most once every
 * 10 seconds (the release script retries 10, 20, 30 and 40 seconds apart).
 */
class ReleaseHookController extends Controller
{
    public const WRONG_TOKENS = 5;

    public const LOCKOUT = 600;

    public const REFRESH_GAP = 10;

    public function __invoke(Request $request): JsonResponse
    {
        $token = (string) config('services.release_hook.token');
        if ($token === '') {
            abort(404);
        }
        $key = 'release-hook-denied:'.$request->ip();
        if (RateLimiter::tooManyAttempts($key, self::WRONG_TOKENS)) {
            abort(429, '', ['Retry-After' => RateLimiter::availableIn($key)]);
        }
        if (! hash_equals($token, (string) $request->bearerToken())) {
            RateLimiter::hit($key, self::LOCKOUT);
            abort(404);
        }

        if (Cache::add('release-hook-refreshed', true, self::REFRESH_GAP)) {
            Releases::refresh();
        }
        $feed = json_decode((string) @file_get_contents(public_path('latest.json')), true);

        return response()->json(['version' => is_array($feed) ? ($feed['version'] ?? null) : null]);
    }
}
