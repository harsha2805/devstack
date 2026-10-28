<?php

namespace Tests\Feature;

use App\Http\Controllers\SiteController;
use App\Support\Releases;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SiteTest extends TestCase
{
    private ?string $savedFeed = null;

    protected function setUp(): void
    {
        parent::setUp();
        // Tests never reach GitHub; a call nothing fakes fails like GitHub being down.
        Http::preventStrayRequests();
        $feed = public_path('latest.json');
        $this->savedFeed = is_file($feed) ? file_get_contents($feed) : null;
        @unlink($feed);
    }

    protected function tearDown(): void
    {
        $feed = public_path('latest.json');
        @unlink($feed);
        if ($this->savedFeed !== null) {
            file_put_contents($feed, $this->savedFeed);
        }
        parent::tearDown();
    }

    private function feed(array $data): void
    {
        file_put_contents(public_path('latest.json'), json_encode($data));
    }

    public function test_home_before_the_first_release(): void
    {
        $this->get('/')->assertOk()
            ->assertSee('Every project folder, served.')
            ->assertSee('Coming soon')
            ->assertSee('Sponsor on GitHub');
    }

    public function test_home_links_the_release_from_the_feed(): void
    {
        $url = 'https://github.com/harsha2805/devstack/releases/download/v0.1.0/devstack_setup_0.1.0.exe';
        $this->feed(['version' => '0.1.0', 'url' => $url, 'notes' => "Trusted HTTPS.\nAI guide per project.", 'signature' => 'x']);

        $this->get('/')->assertOk()
            ->assertSee('Download devstack 0.1.0')
            ->assertSee($url, false)
            ->assertDontSee('Coming soon');
        $this->get('/download')->assertRedirect($url);
    }

    public function test_a_feed_without_https_is_ignored(): void
    {
        $this->feed(['version' => '0.1.0', 'url' => 'http://example.com/setup.exe']);

        $this->get('/')->assertOk()->assertSee('Coming soon')->assertDontSee('example.com');
        $this->get('/download')->assertRedirect(SiteController::REPO.'/releases');
    }

    public function test_changelog_renders_versions_and_escapes_html(): void
    {
        // Before GitHub is asked: the CHANGELOG.md copy.
        $response = $this->get('/changelog')->assertOk()->assertSee('Changelog');
        $html = $response->getContent();
        $this->assertStringContainsString('<article class="release" id="v0.1.0"', $html);
        $this->assertStringContainsString('October 27, 2026', $html, 'dates read as words');
        $this->assertStringNotContainsString('build-release.ps1', $html, 'maintainer notes above the first version should be dropped');
    }

    public function test_changelog_layout_from_changelog_json(): void
    {
        $many = implode("\n", array_map(fn ($i) => "- Fix number $i.", range(1, 6)));
        Http::fake([
            Releases::API.'*' => Http::response([['tag_name' => 'v0.3.0', 'assets' => [$this->asset('v0.3.0', 'changelog.json')]]]),
            Releases::DOWNLOADS.'v0.3.0/changelog.json' => Http::response(['versions' => [
                ['version' => '0.3.1', 'date' => '', 'markdown' => "### Added\n- Coming next."],
                ['version' => '0.3.0', 'date' => '2026-11-02', 'markdown' => "### Added\n- **Terminal.** <script>alert(1)</script>\n\n### Fixed\n$many"],
            ]]),
        ]);
        $this->feed(['version' => '0.3.0', 'url' => Releases::DOWNLOADS.'v0.3.0/devstack_setup_0.3.0.exe']);

        $html = $this->get('/changelog')->assertOk()->getContent();
        $this->assertStringContainsString('<a class="release-tag" id="v0.3.0-title" href="#v0.3.0">0.3.0</a>', $html);
        $this->assertStringContainsString('<time datetime="2026-11-02">November 2, 2026</time>', $html);
        $this->assertMatchesRegularExpression('/id="v0\.3\.1".*?Unreleased/s', $html);
        $this->assertMatchesRegularExpression('/id="v0\.3\.0".*?release-latest">Latest/s', $html, 'the version the feed offers is marked latest');
        // Six fixes collapse behind their count; one addition doesn't.
        $this->assertMatchesRegularExpression('/<details class="release-group">\s*<summary><span class="release-group-title">Fixed<\/span> <span class="release-count">6<\/span>/', $html);
        $this->assertStringContainsString('<h3>Added</h3>', $html);
        $this->assertStringNotContainsString('<script>alert(1)</script>', $html);
    }

    public function test_privacy_page(): void
    {
        $this->get('/privacy')->assertOk()->assertSee('devstack.holostack.in/latest.json')->assertSee('Check for updates automatically');
        $this->get('/')->assertSee(route('privacy'), false)->assertSee('Run anyway')->assertDontSee('SignPath');
        $this->get('/code-signing')->assertRedirect('/');
    }

    public function test_sponsor_redirects_to_github_sponsors(): void
    {
        $this->get('/sponsor')->assertRedirect(SiteController::SPONSOR);
    }

    private function release010(): void
    {
        $this->feed(['version' => '0.1.0', 'url' => 'https://github.com/harsha2805/devstack/releases/download/v0.1.0/devstack_setup_0.1.0.exe']);
    }

    private function asset(string $tag, string $name, int $count = 0): array
    {
        return ['name' => $name, 'download_count' => $count, 'browser_download_url' => Releases::DOWNLOADS."$tag/$name"];
    }

    private function githubReleases(): array
    {
        return [
            ['tag_name' => 'v0.3.0-rc1', 'prerelease' => true, 'assets' => [$this->asset('v0.3.0-rc1', 'devstack_setup_0.3.0.exe', 7)]],
            ['tag_name' => 'v0.2.0', 'assets' => [
                $this->asset('v0.2.0', 'devstack_setup_0.2.0.exe', 1000),
                $this->asset('v0.2.0', 'devstack_0.2.0_portable.zip', 200),
                $this->asset('v0.2.0', 'SHA256SUMS.txt', 999),
            ]],
            ['tag_name' => 'v0.1.0', 'assets' => [$this->asset('v0.1.0', 'devstack_setup_0.1.0.exe', 35)]],
        ];
    }

    /** GitHub with a newest release v0.2.3 that carries its signed feed and changelog. */
    private function fakeRelease023(array $feed = []): void
    {
        $tag = 'v0.2.3';
        Http::fake([
            Releases::API.'*' => Http::response([['tag_name' => $tag, 'assets' => [
                $this->asset($tag, 'devstack_setup_0.2.3.exe', 10),
                $this->asset($tag, 'devstack_0.2.3_portable.zip', 2),
                $this->asset($tag, 'SHA256SUMS.txt'),
                $this->asset($tag, 'latest.json', 500),
                $this->asset($tag, 'changelog.json'),
            ]]]),
            Releases::DOWNLOADS."$tag/latest.json" => Http::response($feed + [
                'version' => '0.2.3', 'url' => Releases::DOWNLOADS."$tag/devstack_setup_0.2.3.exe",
                'sha256' => str_repeat('a', 64), 'notes' => 'php and composer per site.', 'signature' => 'c2ln',
            ]),
            Releases::DOWNLOADS."$tag/changelog.json" => Http::response(['versions' => [
                ['version' => '0.2.3', 'date' => '2026-10-28', 'markdown' => "### Added\n- **Shims.** php and composer per site."],
                ['version' => '0.2.2', 'date' => '2026-10-28', 'markdown' => "### Fixed\n- **ODBC.** The right driver."],
            ]]),
        ]);
    }

    public function test_home_shows_the_download_total_from_github(): void
    {
        Http::fake([Releases::API.'*' => Http::response($this->githubReleases())]);
        $this->release010();

        // Installers and zips of published releases; checksums and prereleases don't count.
        $this->get('/')->assertOk()->assertSee('1,235')->assertSee('downloads so far');
        $this->travel(23)->hours();
        $this->get('/')->assertOk()->assertSee('1,235');
        Http::assertSentCount(1); // cached: GitHub is asked at most once a day
    }

    public function test_github_failures_never_break_the_page(): void
    {
        Http::fake([Releases::API.'*' => Http::sequence()
            ->push($this->githubReleases())
            ->push('rate limited', 403)]);
        $this->release010();
        $this->get('/')->assertSee('1,235');

        // A day later GitHub refuses: the last known total stays.
        $this->travel(25)->hours();
        $this->get('/')->assertOk()->assertSee('1,235');
        Http::assertSentCount(2); // it did ask again, and was refused

        // The failure is remembered for 10 minutes rather than retried on every view.
        $this->get('/')->assertOk();
        Http::assertSentCount(2);
    }

    public function test_the_release_hook_publishes_the_new_version(): void
    {
        config(['services.release_hook.token' => 'hook-secret']);
        $this->fakeRelease023();
        $this->release010();

        $this->post('/hooks/release')->assertNotFound();
        $this->post('/hooks/release', [], ['Authorization' => 'Bearer wrong'])->assertNotFound();
        Http::assertNothingSent();

        $this->post('/hooks/release', [], ['Authorization' => 'Bearer hook-secret'])->assertOk()->assertJson(['version' => '0.2.3']);
        $feed = json_decode(file_get_contents(public_path('latest.json')), true);
        $this->assertSame('0.2.3', $feed['version'], 'public/latest.json serves the new signed feed');
        $this->assertSame('c2ln', $feed['signature']);

        $this->get('/')->assertSee('Download devstack 0.2.3')
            ->assertSee(Releases::DOWNLOADS.'v0.2.3/devstack_0.2.3_portable.zip', false)
            ->assertSee(Releases::DOWNLOADS.'v0.2.3/SHA256SUMS.txt', false)
            ->assertSee('12'); // downloads: installer and zip, not latest.json
        $this->get('/download/portable')->assertRedirect(Releases::DOWNLOADS.'v0.2.3/devstack_0.2.3_portable.zip');
        $this->get('/changelog')->assertSee('October 28, 2026')->assertSee('php and composer per site.');
        Http::assertSentCount(3); // the release list, its latest.json and changelog.json; pages use the cache
    }

    public function test_the_release_hook_is_rate_limited(): void
    {
        config(['services.release_hook.token' => 'hook-secret']);
        $this->fakeRelease023();
        $this->release010();
        $ok = ['Authorization' => 'Bearer hook-secret'];

        // Back-to-back valid calls ask GitHub once; after the gap, again.
        $this->post('/hooks/release', [], $ok)->assertOk();
        $this->post('/hooks/release', [], $ok)->assertOk()->assertJson(['version' => '0.2.3']);
        Http::assertSentCount(3);
        $this->travel(11)->seconds();
        $this->post('/hooks/release', [], $ok)->assertOk();
        Http::assertSentCount(6);

        // Guessing: five wrong tokens lock the IP out, even with the right one, for 10 minutes.
        $this->travel(2)->minutes(); // a fresh minute for the route's own limit
        foreach (range(1, 5) as $i) {
            $this->post('/hooks/release', [], ['Authorization' => "Bearer guess-$i"])->assertNotFound();
        }
        $this->post('/hooks/release', [], $ok)->assertStatus(429)->assertHeader('Retry-After');
        $this->travel(11)->minutes();
        $this->post('/hooks/release', [], $ok)->assertOk();
    }

    public function test_the_release_hook_is_off_without_a_token(): void
    {
        config(['services.release_hook.token' => null]);
        Http::fake();
        $this->post('/hooks/release', [], ['Authorization' => 'Bearer '])->assertNotFound();
        Http::assertNothingSent();
    }

    public function test_the_feed_never_steps_back(): void
    {
        config(['services.release_hook.token' => 'hook-secret']);
        $this->feed(['version' => '0.3.0', 'url' => Releases::DOWNLOADS.'v0.3.0/devstack_setup_0.3.0.exe', 'signature' => 'x']);
        $this->fakeRelease023();
        $this->post('/hooks/release', [], ['Authorization' => 'Bearer hook-secret'])->assertJson(['version' => '0.3.0']);
    }

    public static function badFeeds(): array
    {
        return [
            'installer elsewhere' => [['url' => 'https://evil.example/setup.exe']],
            'unsigned' => [['signature' => '']],
            'not the release it came with' => [['version' => '0.2.4']],
        ];
    }

    #[\PHPUnit\Framework\Attributes\DataProvider('badFeeds')]
    public function test_a_bad_feed_is_never_served(array $bad): void
    {
        config(['services.release_hook.token' => 'hook-secret']);
        $this->release010();
        $this->fakeRelease023($bad);
        $this->post('/hooks/release', [], ['Authorization' => 'Bearer hook-secret'])->assertJson(['version' => '0.1.0']);
    }

    public function test_portable_link_before_github_was_asked(): void
    {
        Http::fake([Releases::API.'*' => Http::response('down', 500)]);
        $this->release010();
        // Every release names its files the same way, so the link works without GitHub.
        $this->get('/')->assertSee(Releases::DOWNLOADS.'v0.1.0/devstack_0.1.0_portable.zip', false);
    }

    public function test_no_total_when_github_was_never_reachable(): void
    {
        Http::fake([Releases::API.'*' => Http::response('down', 500)]);
        $this->release010();

        $this->get('/')->assertOk()->assertSee('Download devstack 0.1.0')->assertDontSee('downloads so far');
    }

    public function test_no_github_call_before_the_first_release(): void
    {
        Http::fake();
        $this->get('/')->assertOk()->assertSee('Coming soon');
        Http::assertNothingSent();
    }
}
