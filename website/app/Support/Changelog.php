<?php

namespace App\Support;

use Illuminate\Support\Carbon;
use Illuminate\Support\Str;
use Throwable;

/**
 * The changelog page's data: one entry per release, each split into its
 * "### Added" / "### Fixed" groups, rendered to HTML.
 */
class Changelog
{
    /** Groups with more items than this start collapsed behind their count. */
    public const COLLAPSE_OVER = 5;

    /**
     * Entries from the newest release's changelog.json, else from CHANGELOG.md
     * (the copy deploy.ps1 ships; locally, the repo's own).
     *
     * @return list<array{version: string, date: string|null, iso: string|null, latest: bool, groups: list<array{title: string, count: int, html: string, collapsed: bool}>}>
     */
    public static function entries(?string $latest): array
    {
        $versions = Releases::changelog() ?? self::fromMarkdown(self::bundled());

        return array_map(fn ($v) => [
            'version' => $v['version'],
            'date' => self::longDate($v['date']),
            'iso' => $v['date'] ?: null,
            'latest' => $v['version'] === $latest,
            'groups' => self::groups($v['markdown']),
        ], $versions);
    }

    /**
     * CHANGELOG.md's "## [x.y.z] — date" sections, as changelog.json has them.
     *
     * @return list<array{version: string, date: string, markdown: string}>
     */
    public static function fromMarkdown(string $markdown): array
    {
        $markdown = str_replace("\r\n", "\n", $markdown);
        preg_match_all('/^## \[(\d+\.\d+\.\d+)\]([^\n]*)$/m', $markdown, $heads, PREG_OFFSET_CAPTURE);
        $out = [];
        foreach ($heads[0] as $i => [$heading, $start]) {
            $bodyStart = $start + strlen($heading);
            $end = $heads[0][$i + 1][1] ?? strlen($markdown);
            preg_match('/\d{4}-\d{2}-\d{2}/', $heads[2][$i][0], $date);
            $out[] = [
                'version' => $heads[1][$i][0],
                'date' => $date[0] ?? '',
                'markdown' => trim(substr($markdown, $bodyStart, $end - $bodyStart)),
            ];
        }

        return $out;
    }

    /** @return list<array{title: string, count: int, html: string, collapsed: bool}> */
    private static function groups(string $markdown): array
    {
        $parts = preg_split('/^### +(.+)$/m', str_replace("\r\n", "\n", $markdown), -1, PREG_SPLIT_DELIM_CAPTURE);
        $groups = [];
        if (trim($parts[0]) !== '') {
            $groups[] = self::group('', $parts[0]); // text before the first "###"
        }
        for ($i = 1; $i + 1 < count($parts); $i += 2) {
            $groups[] = self::group(trim($parts[$i]), $parts[$i + 1]);
        }

        return $groups;
    }

    private static function group(string $title, string $markdown): array
    {
        $count = preg_match_all('/^[-*] /m', $markdown);

        return [
            'title' => $title,
            'count' => $count,
            'html' => Str::markdown(trim($markdown), ['html_input' => 'escape', 'allow_unsafe_links' => false]),
            'collapsed' => $title !== '' && $count > self::COLLAPSE_OVER,
        ];
    }

    private static function longDate(string $date): ?string
    {
        if ($date === '') {
            return null;
        }
        try {
            return Carbon::parse($date)->format('F j, Y');
        } catch (Throwable) {
            return $date;
        }
    }

    private static function bundled(): string
    {
        $path = collect([resource_path('markdown/CHANGELOG.md'), base_path('../CHANGELOG.md')])->first(fn ($p) => is_file($p));

        return $path ? (string) file_get_contents($path) : '';
    }
}
