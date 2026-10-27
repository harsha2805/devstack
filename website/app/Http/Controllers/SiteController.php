<?php

namespace App\Http\Controllers;

use App\Support\Downloads;
use Illuminate\Http\RedirectResponse;
use Illuminate\Support\Str;
use Illuminate\View\View;

/**
 * The devstack website. It has no database: the current release comes from
 * public/latest.json (the same signed feed the app reads) and the changelog
 * from resources/markdown/CHANGELOG.md. deploy.ps1 copies both in from the
 * repository before each upload.
 */
class SiteController extends Controller
{
    public const REPO = 'https://github.com/harsha2805/devstack';

    public const SPONSOR = 'https://github.com/sponsors/harsha2805';

    public function home(): View
    {
        $release = $this->release();

        return view('home', ['release' => $release, 'downloads' => $release ? Downloads::total() : null]);
    }

    public function changelog(): View
    {
        // deploy.ps1 copies the changelog in; locally, read the repo's own.
        $path = collect([resource_path('markdown/CHANGELOG.md'), base_path('../CHANGELOG.md')])->first(fn ($p) => is_file($p));
        $markdown = $path ? file_get_contents($path) : '';
        // The page has its own heading and intro; start at the first version.
        $markdown = preg_replace('/\A.*?(?=^## )/ms', '', $markdown) ?? '';
        // "## [0.1.0] — date" reads better on a web page without the brackets.
        $markdown = preg_replace('/^## \[([^\]]+)\]/m', '## $1', $markdown) ?? '';

        return view('changelog', [
            'html' => Str::markdown($markdown, ['html_input' => 'escape', 'allow_unsafe_links' => false]),
            'release' => $this->release(),
        ]);
    }

    /** Short link for the newest installer, e.g. from the README. */
    public function download(): RedirectResponse
    {
        $release = $this->release();

        return redirect()->away($release['url'] ?? self::REPO.'/releases');
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
