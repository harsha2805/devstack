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

1. `.\publish-release.ps1 -Version 0.2.0` tags the pushed commit `v0.2.0` and pushes the tag. GitHub builds the
   release ([release.yml](../.github/workflows/release.yml)) in a few minutes, ending as a draft release.
2. `.\publish-release.ps1 -Version 0.2.0` again, once the draft is there. It downloads the files into
   `dist\release\`, checks them against `SHA256SUMS.txt`, writes and signs
   `latest.json`, and publishes the release:

   | File | What |
   |---|---|
   | `devstack_setup_<version>.exe` | per-user installer (no admin), adds devstack to the user PATH |
   | `devstack_<version>_portable.zip` | both exes, for people who don't want an installer |
   | `SHA256SUMS.txt` | hashes of the two files above |
   | `latest.json` | the update feed, signed (goes on the website, not the release) |

   The update notes in Settings → Updates are the changelog section's bold lead-ins (`- **Folder picker.** ...`);
   `-Notes` overrides them.
3. `website\deploy.ps1` packages the website with the new `latest.json` and changelog; upload it as described in
   [WEBSITE.md](WEBSITE.md). It only includes `latest.json` once the installer's GitHub URL is live, so installed
   apps never see an update before its file exists.

Installers live on GitHub Releases (free bandwidth, visible history); only `latest.json` is on the website.

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
