<?php

namespace Tests\Feature;

use App\Http\Controllers\SiteController;
use App\Support\Downloads;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class SiteTest extends TestCase
{
    private ?string $savedFeed = null;

    protected function setUp(): void
    {
        parent::setUp();
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
        $response = $this->get('/changelog')->assertOk()->assertSee('Changelog');
        $this->assertMatchesRegularExpression('/<h2>0\.1\.0 /', $response->getContent());
        $this->assertStringNotContainsString('build-release.ps1', $response->getContent(), 'maintainer notes above the first version should be dropped');
    }

    public function test_code_signing_policy_and_privacy_pages(): void
    {
        // SignPath Foundation asks for this wording, linked from the home and download pages.
        $this->get('/code-signing')->assertOk()
            ->assertSee('Free code signing provided by', false)
            ->assertSee('certificate by', false)
            ->assertSee('Approvers');
        $this->get('/privacy')->assertOk()->assertSee('devstack.holostack.in/latest.json')->assertSee('Check for updates automatically');
        $this->get('/')->assertSee('Code signing policy')->assertSee(route('privacy'), false);
    }

    public function test_sponsor_redirects_to_github_sponsors(): void
    {
        $this->get('/sponsor')->assertRedirect(SiteController::SPONSOR);
    }

    private function release010(): void
    {
        $this->feed(['version' => '0.1.0', 'url' => 'https://github.com/harsha2805/devstack/releases/download/v0.1.0/devstack_setup_0.1.0.exe']);
    }

    private function githubReleases(): array
    {
        return [
            ['tag_name' => 'v0.2.0', 'assets' => [
                ['name' => 'devstack_setup_0.2.0.exe', 'download_count' => 1000],
                ['name' => 'devstack_0.2.0_portable.zip', 'download_count' => 200],
                ['name' => 'SHA256SUMS.txt', 'download_count' => 999],
            ]],
            ['tag_name' => 'v0.1.0', 'assets' => [['name' => 'devstack_setup_0.1.0.exe', 'download_count' => 35]]],
        ];
    }

    public function test_home_shows_the_download_total_from_github(): void
    {
        Http::fake([Downloads::API.'*' => Http::response($this->githubReleases())]);
        $this->release010();

        // Installers and zips across releases; the checksum file doesn't count.
        $this->get('/')->assertOk()->assertSee('1,235')->assertSee('downloads so far');
        $this->get('/')->assertOk()->assertSee('1,235');
        Http::assertSentCount(1); // cached: GitHub is asked at most once an hour
    }

    public function test_github_failures_never_break_the_page(): void
    {
        Http::fake([Downloads::API.'*' => Http::sequence()
            ->push($this->githubReleases())
            ->push('rate limited', 403)]);
        $this->release010();
        $this->get('/')->assertSee('1,235');

        // An hour later GitHub refuses: the last known total stays.
        $this->travel(61)->minutes();
        $this->get('/')->assertOk()->assertSee('1,235');
        Http::assertSentCount(2); // it did ask again, and was refused

        // The failure is remembered for 10 minutes rather than retried on every view.
        $this->get('/')->assertOk();
        Http::assertSentCount(2);
    }

    public function test_no_total_when_github_was_never_reachable(): void
    {
        Http::fake([Downloads::API.'*' => Http::response('down', 500)]);
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
