# Code signing

Windows SmartScreen warns about unsigned downloads ("Windows protected your PC"). devstack's releases are signed
through the [SignPath Foundation](https://signpath.org), which gives open source projects a free code signing
certificate. The certificate belongs to SignPath Foundation, and its private key never leaves SignPath: GitHub
Actions builds the files, SignPath signs them after a person approves the request, and the signed files go back
to the build.

This is separate from the update feed's signature (`latest.json`, [RELEASING.md](RELEASING.md)), which uses
devstack's own offline key.

## How a signed release is built

`publish-release.ps1 -Version x.y.z` pushes the tag `vx.y.z`, which starts
[`.github/workflows/release.yml`](../.github/workflows/release.yml) on a GitHub-hosted Windows runner:

1. Runs `go test ./...` and `build.ps1`, which writes the version, product name and publisher into both exes.
2. Uploads the exes as a build artifact and submits it to SignPath (configuration `exes`). The run waits until an
   approver approves the request on signpath.io, then takes the signed exes back.
3. Builds the installer around the signed exes and submits that (configuration `installer`), a second approval.
4. Checks every signature, builds the portable zip and `SHA256SUMS.txt`, and creates a **draft** GitHub release.

Running `publish-release.ps1 -Version x.y.z` again downloads the draft, checks the hashes and that the installer and
both exes in the zip are validly signed, signs `latest.json` with the offline key and publishes the release.

Without the SignPath settings below, the workflow still builds the same files, unsigned;
`publish-release.ps1 -AllowUnsigned` publishes such a build.

If a tag's build fails, fix the cause, then delete the tag and start again:

```powershell
git push --delete origin v0.2.0; git tag -d v0.2.0
.\publish-release.ps1 -Version 0.2.0
```

## Applying to SignPath Foundation (once)

What they check (see [their terms](https://signpath.org/terms)), and where devstack meets it:

| Requirement | devstack |
|---|---|
| OSI license, no proprietary parts | MIT; nginx, PHP and MySQL are downloaded at run time, not shipped |
| Already released, functionality documented | v0.1.0 on GitHub; README and devstack.holostack.in |
| Built from source on a verifiable build system | `release.yml` on GitHub-hosted runners |
| "Code signing policy" on the home and download pages, with the attribution sentence and team roles | README section, `devstack.holostack.in/code-signing` (linked from the home page and footer), and every GitHub release's notes |
| Privacy policy, shown during installation, transfers can be turned off | `devstack.holostack.in/privacy`; Setup shows `installer/privacy.txt`; Settings → Updates turns the update check off |
| Uninstaller | Setup registers one in Windows' Apps list |
| Product name and version the same on every signed file | `devstack` and `x.y.z`, from `build.ps1` and `installer/devstack.iss` |
| Multi-factor authentication for everyone on the team | turn it on for GitHub and for the SignPath account |

1. Turn on two-factor authentication on GitHub (Settings → Password and authentication) if it isn't already.
2. Deploy the website (`website\deploy.ps1`) so `/code-signing` and `/privacy` are live, and push the README.
3. Apply at [signpath.org](https://signpath.org) (Apply / "Get started"), with the repository
   `https://github.com/harsha2805/devstack`, the website, and the release page. Use the GitHub account's email.
4. They review the project and reply by email. That can take a few weeks.

## Setting it up once accepted

In SignPath (app.signpath.io), in the project SignPath Foundation created:

1. **Trusted build system:** link GitHub.com to the project and the `harsha2805/devstack` repository, following
   [SignPath's GitHub guide](https://docs.signpath.io/trusted-build-systems/github).
2. **Artifact configurations:** add two, pasting the XML from this repository:
   - slug `exes`: [`.signpath/artifact-configurations/exes.xml`](../.signpath/artifact-configurations/exes.xml)
   - slug `installer`: [`.signpath/artifact-configurations/installer.xml`](../.signpath/artifact-configurations/installer.xml)
3. **Signing policies:** the project normally has `test-signing` (a test certificate, not trusted by Windows) and
   `release-signing` (the real one, each request approved by hand). Make yourself the approver for
   `release-signing`.
4. **API token:** create a CI user (or use your own) with the Submitter role on both policies, and generate an API
   token for it.
5. Note the **organization ID** (shown in the organization settings) and the **project slug**.

In GitHub, repository Settings → Secrets and variables → Actions:

| Kind | Name | Value |
|---|---|---|
| Secret | `SIGNPATH_API_TOKEN` | the API token |
| Variable | `SIGNPATH_ORGANIZATION_ID` | the organization ID |
| Variable | `SIGNPATH_PROJECT_SLUG` | the project slug, e.g. `devstack` |
| Variable | `SIGNPATH_SIGNING_POLICY_SLUG` | `test-signing` for a first try, then `release-signing` |

Inno Setup pads the installer's version strings with spaces (`0.1.1` followed by blanks). If SignPath rejects
the installer's product name or version as not matching, that padding is the cause; ask SignPath support to compare
them trimmed rather than dropping the check.

Try it with `test-signing` on a throwaway tag (for example `v0.0.99`, with a `## [0.0.99]` changelog section on a
branch), delete that draft and tag, then switch the variable to `release-signing`.

## What users see

A signed installer shows **SignPath Foundation** as the verified publisher, and the file's details show devstack
and HoloStack. SmartScreen reputation still builds up over the first downloads, but signed files reach it much
faster than unsigned ones, which never do.

The uninstaller (`unins000.exe`) is written by Setup at install time and isn't signed. Windows doesn't show
SmartScreen warnings for it.

If a file is flagged anyway, it can be submitted to Microsoft at
<https://www.microsoft.com/wdsi/filesubmission> as "Software developer".
