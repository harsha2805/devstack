# Releasing devstack

A release is an installer, a portable zip and a signed update feed. The installer and zip go on GitHub Releases;
the feed goes on `https://devstack.holostack.in/latest.json`.

## Once per machine

- Inno Setup 6, for local builds: `winget install -e --id JRSoftware.InnoSetup --scope user`
- The GitHub CLI, signed in: `gh auth login`
- The release signing key (below) at `%APPDATA%\devstack-release\update-signing-key.pem`, restored from backup.

## Try it locally

Every feature or fix gets its line in `CHANGELOG.md` when it lands, under the version it will ship in. To try the
installer before releasing:

```powershell
.\build-release.ps1 -Version 0.2.0
```

It refuses a version without a `## [0.2.0]` section in `CHANGELOG.md`, runs the tests, builds both exes with the
version stamped in, and writes an unsigned installer, portable zip and `SHA256SUMS.txt` to `dist\release\`. Older
release files move to `dist\release\old\<version>\`. These local files are never published.

## Publish

Released files are built by GitHub Actions, so anyone can trace them back to this repository. They aren't
code-signed, so SmartScreen warns on first run ("More info", then "Run anyway"). Commit and push, then:

`.\publish-release.ps1 -Version 0.2.0` tags the pushed commit `v0.2.0`, pushes the tag and waits. GitHub builds
and tests it ([release.yml](../.github/workflows/release.yml)), signs `latest.json` with the release key, publishes
the release and calls the website, which serves the new version within seconds. No website upload is needed.

| File on the release | What |
|---|---|
| `devstack_setup_<version>.exe` | per-user installer (no admin), adds devstack to the user PATH |
| `devstack_<version>_portable.zip` | both exes and the php/composer shims, for people who don't want an installer |
| `SHA256SUMS.txt` | hashes of the two files above |
| `latest.json` | the update feed, signed; the website copies it to `/latest.json`, where the app reads it |
| `changelog.json` | `CHANGELOG.md` as JSON, for the website's changelog page |

The update notes in Settings → Updates are the changelog section's bold lead-ins (`- **Folder picker.** ...`);
`-Notes` overrides them in the local flow below.

### Secrets and protections (once)

The secrets live in the `release` environment (Settings → Environments), which GitHub only lets run from `v*`
tags, so no other workflow, branch or pull request can read them:

| `release` environment secret | Value |
|---|---|
| `DEVSTACK_UPDATE_KEY` | the release key's PEM text (`%APPDATA%\devstack-release\update-signing-key.pem`) |
| `RELEASE_HOOK_TOKEN` | a random token; the same value as `RELEASE_HOOK_TOKEN` in `website\.env.production` |

```powershell
Get-Content -Raw "$env:APPDATA\devstack-release\update-signing-key.pem" | gh secret set DEVSTACK_UPDATE_KEY --env release --repo harsha2805/devstack
```

Rulesets (Settings → Rules): `main` can't be force-pushed or deleted, and only admins can create, move or delete
`v*` tags, the trigger for a signed release. Actions asks for approval before running workflows from outside
contributors' pull requests, and the default workflow token is read-only.

With the key in GitHub, anyone who can push a `v*` tag or change the workflow on `main` can sign an update that
every installed devstack accepts. Keep two-factor authentication on, and don't give others write access lightly.
To sign only on this machine again, delete the `DEVSTACK_UPDATE_KEY` secret: the workflow then stops at a draft
and `publish-release.ps1` signs, attaches `latest.json`, publishes and calls the website itself (it reads the
token from `website\.env.production`).

Without `RELEASE_HOOK_TOKEN` (or if the call fails) the website still picks the release up on its own within a day.
Installers live on GitHub Releases (free bandwidth, visible history); the website only copies the feed.

## How updates work

The app reads `https://devstack.holostack.in/latest.json`:

```json
{
  "version": "0.2.0",
  "url": "https://github.com/harsha2805/devstack/releases/download/v0.2.0/devstack_setup_0.2.0.exe",
  "sha256": "<SHA-256 of the installer>",
  "notes": "Shown in Settings → Updates.",
  "page": "https://devstack.holostack.in",
  "signature": "<Ed25519 signature, added by tools/release>"
}
```

- It checks 30 seconds after launch and then once a day, unless "Check for updates automatically" is off.
  Settings → Updates also has "Check now".
- Nothing installs on its own. **Update now** downloads the installer, checks its SHA-256, stops running
  servers, starts Setup silently and quits. Setup reopens devstack, which starts the servers again.
- Portable copies open `page` instead.
- `url` and `page` must be `https://`. The app refuses a feed without a valid signature from a key in
  `update.TrustedKeys` (`internal/update/update.go`).

Setup, interactive or silent, first runs `devstack-app.exe --quit` from the old install. That closes the app,
stops the servers and waits, so no program file is in use when it is replaced. The uninstaller does the same,
removes the PATH entry, the sign-in entry and `%LOCALAPPDATA%\devstack`, and keeps the stack folder.

## Release signing key

The signature covers `version`, `url`, `sha256`, `page` and `notes`, so **never edit `latest.json` by hand after
signing**. Re-run `go run ./tools/release sign dist\release\latest.json` instead.

- The private key lives outside the repo at `%APPDATA%\devstack-release\update-signing-key.pem` (or the path
  in `DEVSTACK_UPDATE_KEY`). **Back it up offline**, for example in a password manager or on an encrypted USB
  drive. Never commit it.
- `go run ./tools/release check` confirms the key matches the app; `publish-release.ps1` signs with it.
- **Rotating:** generate a new key (`keygen` with a fresh `DEVSTACK_UPDATE_KEY` path), add its public key to
  `update.TrustedKeys` next to the old one, and ship a release signed with the old key. Sign with the new key
  from the release after that.
- **Lost key:** released apps can't verify new feeds. Add a new key as above; existing users have to install
  that release by hand.

## Testing an installer without touching your own setup

Build the installer with a separate settings folder and point the app at it:

```powershell
$t = "$env:TEMP\devstack-test"
& "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe" /DAppVersion=0.0.1 /DBin=..\dist "/DAppData=$t\appdata" "/O$t" installer\devstack.iss
$env:DEVSTACK_APPDATA = "$t\appdata"
& "$t\devstack_setup_0.0.1.exe" /SILENT "/DIR=$t\app"
```

The install still registers itself under "devstack" (uninstall entry, user PATH, sign-in entry), so uninstall it
afterwards from the same shell.
