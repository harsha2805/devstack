<?php

namespace App\Support;

use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;
use Throwable;

/**
 * What the site knows about devstack's releases, from GitHub Releases: the
 * signed update feed (latest.json) and changelog.json the release workflow
 * attaches, each release's files, and the download total.
 *
 * The release workflow calls POST /hooks/release when it publishes, which
 * refreshes at once. Otherwise GitHub is asked at most once a day (it allows
 * 60 unauthenticated API calls an hour per IP, and a shared host's IP is often
 * shared); after a failure the last known data is kept and the next try waits
 * 10 minutes, so a page view never stalls on GitHub more than once.
 *
 * The feed is written to public/latest.json, where the app (and the web server,
 * as a plain file) reads it. Only a newer, well-formed feed signed with one of
 * KEYS replaces it, so the home page never links an installer the app would
 * refuse; the app checks the signature again itself. deploy.ps1 ships the
 * current one as a start.
 */
class Releases
{
    public const API = 'https://api.github.com/repos/harsha2805/devstack/releases';

    public const DOWNLOADS = 'https://github.com/harsha2805/devstack/releases/download/';

    /**
     * The release signing keys (base64 Ed25519 public keys): the same list as
     * TrustedKeys in internal/update/update.go, which a Go test compares.
     */
    public const KEYS = [
        'n40UCrPqQCdg6/uEsdNj83sUDJp7kFTj3EpIf1tpFsQ=', // 2026-10-26
    ];

    /** @var list<string>|null tests sign with keys of their own */
    public static ?array $keys = null;

    private const KEY = 'github-releases';

    private const FRESH = 86400;

    private const RETRY = 600;

    /**
     * The cached data, refreshed first when it is a day old.
     *
     * @return array{downloads: int|null, files: array<string, array<string, string>>, changelog: list<array{version: string, date: string, markdown: string}>|null}|null
     */
    public static function data(): ?array
    {
        $cached = Cache::get(self::KEY); // ['data' => array|null, 'next' => unix time]
        if (is_array($cached) && $cached['next'] > now()->getTimestamp()) {
            return $cached['data'];
        }
        // One visitor refreshes; the rest get the last data rather than each
        // waiting on GitHub too.
        if (! Cache::add(self::KEY.':refreshing', true, 60)) {
            return is_array($cached) ? $cached['data'] : null;
        }
        try {
            return self::refresh();
        } finally {
            Cache::forget(self::KEY.':refreshing');
        }
    }

    /** Asks GitHub now (the release hook); keeps the last data if it can't. */
    public static function refresh(): ?array
    {
        $previous = Cache::get(self::KEY)['data'] ?? null;
        $data = self::fetch();
        if ($data === null) {
            Cache::forever(self::KEY, ['data' => $previous, 'next' => now()->getTimestamp() + self::RETRY]);

            return $previous;
        }
        Cache::forever(self::KEY, ['data' => $data, 'next' => now()->getTimestamp() + self::FRESH]);

        return $data;
    }

    /** Downloads of installers and zips across releases (in-app updates fetch the same installer). */
    public static function downloads(): ?int
    {
        return self::data()['downloads'] ?? null;
    }

    /**
     * A release's files by name, e.g. "devstack_0.2.3_portable.zip" => its URL.
     * Falls back to the names every release uses when GitHub hasn't been asked.
     *
     * @return array<string, string>
     */
    public static function files(string $version): array
    {
        $files = self::data()['files']['v'.$version] ?? null;
        if ($files) {
            return $files;
        }
        $base = self::DOWNLOADS.'v'.$version.'/';

        return collect(["devstack_setup_$version.exe", "devstack_{$version}_portable.zip", 'SHA256SUMS.txt'])
            ->mapWithKeys(fn ($name) => [$name => $base.$name])->all();
    }

    /** @return list<array{version: string, date: string, markdown: string}>|null */
    public static function changelog(): ?array
    {
        return self::data()['changelog'] ?? null;
    }

    private static function fetch(): ?array
    {
        try {
            $response = self::http()->get(self::API, ['per_page' => 100]);
            if (! $response->successful() || ! is_array($response->json())) {
                return null;
            }
            $releases = collect($response->json())->filter(fn ($r) => is_array($r) && empty($r['draft']) && empty($r['prerelease']));

            $files = [];
            foreach ($releases as $r) {
                $files[(string) ($r['tag_name'] ?? '')] = collect($r['assets'] ?? [])
                    ->filter(fn ($a) => str_starts_with((string) ($a['browser_download_url'] ?? ''), self::DOWNLOADS))
                    ->mapWithKeys(fn ($a) => [(string) $a['name'] => (string) $a['browser_download_url']])->all();
            }

            // The highest version, not GitHub's first entry: that is ordered by
            // date, so a fix released for an older line would come first.
            $latest = $releases->sort(fn ($a, $b) => version_compare(
                ltrim((string) ($b['tag_name'] ?? ''), 'v'), ltrim((string) ($a['tag_name'] ?? ''), 'v'),
            ))->first();
            $changelog = null;
            if ($latest) {
                $assets = $files[(string) $latest['tag_name']] ?? [];
                if (isset($assets['latest.json'])) {
                    try {
                        self::storeFeed($assets['latest.json'], ltrim((string) $latest['tag_name'], 'v'));
                    } catch (Throwable $e) {
                        report($e); // an unwritable latest.json must not cost the page its downloads and changelog
                    }
                }
                if (isset($assets['changelog.json'])) {
                    $changelog = self::fetchChangelog($assets['changelog.json']);
                }
            }

            return [
                'downloads' => (int) $releases
                    ->flatMap(fn ($r) => $r['assets'] ?? [])
                    ->filter(fn ($a) => preg_match('/\.(exe|zip)$/i', $a['name'] ?? ''))
                    ->sum('download_count'),
                'files' => $files,
                'changelog' => $changelog,
            ];
        } catch (Throwable $e) {
            report($e);

            return null;
        }
    }

    /** Puts the release's feed in public/latest.json if it is newer than what is there. */
    private static function storeFeed(string $url, string $version): void
    {
        $response = self::http()->get($url);
        $feed = $response->successful() ? $response->json() : null;
        if (! is_array($feed) || ($feed['version'] ?? null) !== $version || empty($feed['signature'])
            || ! str_starts_with((string) ($feed['url'] ?? ''), self::DOWNLOADS)) {
            return;
        }
        if (! self::signed($feed)) {
            report(new \RuntimeException("latest.json of $version failed its signature check; public/latest.json was kept"));

            return;
        }
        $path = public_path('latest.json');
        $current = is_file($path) ? json_decode((string) file_get_contents($path), true) : null;
        if (is_array($current) && version_compare((string) ($current['version'] ?? '0'), $version, '>')) {
            return; // never step back
        }
        $body = $response->body();
        if (is_file($path) && file_get_contents($path) === $body) {
            return;
        }
        $tmp = $path.'.tmp';
        file_put_contents($tmp, $body);
        rename($tmp, $path);
    }

    /**
     * Whether a feed carries a valid signature from one of the keys. The
     * signed bytes are update.Payload's: every field the app acts on, one per
     * line, notes last.
     */
    public static function signed(array $feed): bool
    {
        if (! function_exists('sodium_crypto_sign_verify_detached')) {
            throw new \RuntimeException("PHP has no sodium extension, so release feeds can't be verified");
        }
        $fields = [];
        foreach (['version', 'url', 'sha256', 'page', 'notes'] as $name) {
            $value = $feed[$name] ?? '';
            if (! is_string($value) || ($name !== 'notes' && preg_match('/[\r\n]/', $value))) {
                return false;
            }
            $fields[$name] = $value;
        }
        $payload = "devstack update feed v1\n"
            ."version={$fields['version']}\n"
            ."url={$fields['url']}\n"
            ."sha256={$fields['sha256']}\n"
            ."page={$fields['page']}\n"
            ."notes={$fields['notes']}";
        $signature = base64_decode((string) ($feed['signature'] ?? ''), true);
        if ($signature === false || strlen($signature) !== SODIUM_CRYPTO_SIGN_BYTES) {
            return false;
        }
        foreach (self::$keys ?? self::KEYS as $key) {
            $public = base64_decode($key, true);
            if ($public !== false && strlen($public) === SODIUM_CRYPTO_SIGN_PUBLICKEYBYTES
                && sodium_crypto_sign_verify_detached($signature, $payload, $public)) {
                return true;
            }
        }

        return false;
    }

    /** @return list<array{version: string, date: string, markdown: string}>|null */
    private static function fetchChangelog(string $url): ?array
    {
        $response = self::http()->get($url);
        $versions = $response->successful() ? ($response->json()['versions'] ?? null) : null;
        if (! is_array($versions)) {
            return null;
        }

        return array_values(array_filter(array_map(fn ($v) => is_array($v) && isset($v['version'], $v['markdown']) ? [
            'version' => (string) $v['version'],
            'date' => (string) ($v['date'] ?? ''),
            'markdown' => (string) $v['markdown'],
        ] : null, $versions)));
    }

    private static function http()
    {
        return Http::connectTimeout(3)->timeout(5)->withHeaders(['Accept' => 'application/vnd.github+json', 'User-Agent' => 'devstack-website']);
    }
}
