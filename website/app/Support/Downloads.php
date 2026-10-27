<?php

namespace App\Support;

use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;
use Throwable;

/**
 * Total downloads of devstack's installers and zips, as counted by GitHub
 * Releases. In-app updates fetch the same installer, so they count too.
 *
 * GitHub allows 60 unauthenticated API calls an hour per IP, and a shared host's
 * IP is often shared, so the total is fetched at most once an hour. If GitHub
 * can't be reached the last known total is kept, and the next try waits 10
 * minutes, so a page view never stalls on GitHub more than once.
 */
class Downloads
{
    public const API = 'https://api.github.com/repos/harsha2805/devstack/releases';

    private const KEY = 'github-downloads';

    private const FRESH = 3600;

    private const RETRY = 600;

    /** The total, or null before anything was ever counted. */
    public static function total(): ?int
    {
        $cached = Cache::get(self::KEY); // ['count' => int|null, 'next' => unix time]
        if (is_array($cached) && $cached['next'] > now()->getTimestamp()) {
            return $cached['count'];
        }

        $count = self::fetch();
        if ($count === null) {
            Cache::forever(self::KEY, ['count' => $cached['count'] ?? null, 'next' => now()->getTimestamp() + self::RETRY]);

            return $cached['count'] ?? null;
        }
        Cache::forever(self::KEY, ['count' => $count, 'next' => now()->getTimestamp() + self::FRESH]);

        return $count;
    }

    private static function fetch(): ?int
    {
        try {
            $response = Http::timeout(3)
                ->withHeaders(['Accept' => 'application/vnd.github+json', 'User-Agent' => 'devstack-website'])
                ->get(self::API, ['per_page' => 100]);
            if (! $response->successful() || ! is_array($response->json())) {
                return null;
            }

            return (int) collect($response->json())
                ->flatMap(fn ($release) => $release['assets'] ?? [])
                ->filter(fn ($asset) => preg_match('/\.(exe|zip)$/i', $asset['name'] ?? ''))
                ->sum('download_count');
        } catch (Throwable) {
            return null;
        }
    }
}
