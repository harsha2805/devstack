<?php

namespace App\Http\Controllers;

use App\Support\Changelog;
use App\Support\Releases;
use Illuminate\Http\RedirectResponse;
use Illuminate\View\View;

/**
 * The devstack website. It has no database: the current release comes from
 * public/latest.json (the same signed feed the app reads) and the changelog
 * from the newest release's changelog.json, both kept current from GitHub by
 * App\Support\Releases. deploy.ps1 ships copies of both, used until then.
 */
class SiteController extends Controller
{
    public const REPO = 'https://github.com/harsha2805/devstack';

    public const SPONSOR = 'https://github.com/sponsors/harsha2805';

    public function home(): View
    {
        $release = $this->release();

        return view('home', [
            'release' => $release,
            'downloads' => $release ? Releases::downloads() : null,
            'files' => $release ? Releases::files($release['version']) : [],
        ]);
    }

    public function changelog(): View
    {
        $release = $this->release();

        return view('changelog', ['entries' => Changelog::entries($release['version'] ?? null), 'release' => $release]);
    }

    /** Short link for the newest installer, e.g. from the README. */
    public function download(): RedirectResponse
    {
        $release = $this->release();

        return redirect()->away($release['url'] ?? self::REPO.'/releases');
    }

    /** Short link for the newest portable zip. */
    public function portable(): RedirectResponse
    {
        $release = $this->release();
        $zip = $release ? (Releases::files($release['version'])["devstack_{$release['version']}_portable.zip"] ?? null) : null;

        return redirect()->away($zip ?? self::REPO.'/releases');
    }

    public function sponsor(): RedirectResponse
    {
        return redirect()->away(self::SPONSOR);
    }

    /**
     * The release advertised by latest.json, or null before the first one.
     *
     * @return array{version: string, url: string, notes: list<string>}|null
     */
    private function release(): ?array
    {
        $path = public_path('latest.json');
        if (! is_file($path)) {
            return null;
        }
        $feed = json_decode((string) file_get_contents($path), true);
        if (! is_array($feed) || empty($feed['version']) || empty($feed['url']) || ! str_starts_with($feed['url'], 'https://')) {
            return null;
        }

        return [
            'version' => (string) $feed['version'],
            'url' => (string) $feed['url'],
            'notes' => array_values(array_filter(array_map('trim', explode("\n", (string) ($feed['notes'] ?? ''))))),
        ];
    }
}
