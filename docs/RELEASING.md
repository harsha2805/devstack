# Releasing devstack

A release is an installer, a portable zip and a signed update feed, uploaded to
`https://devstack.holostack.in/`.

## Once per machine

- Inno Setup 6: `winget install -e --id JRSoftware.InnoSetup --scope user`
- The release signing key (below) at `%APPDATA%\devstack-release\update-signing-key.pem`, restored from backup.

## Build

```powershell
.\build-release.ps1 -Version 0.2.0 -Notes "Folder picker. Title bar follows light and dark mode."
```

It checks the signing key, runs the tests, builds both exes with the version stamped in, and writes to
`dist\release\`:

| File | What |
|---|---|
| `devstack_setup_<version>.exe` | per-user installer (no admin), adds devstack to the user PATH |
| `devstack_<version>_portable.zip` | both exes, for people who don't want an installer |
| `SHA256SUMS.txt` | hashes of the two files above |
| `latest.json` | the update feed, signed |

Older release files move to `dist\release\old\<version>\`.

## Publish

1. Upload the installer and the zip to `https://devstack.holostack.in/`.
2. Upload `latest.json` **last**, so the feed never points at a file that isn't there yet.
3. Tag the commit: `git tag v<version>` and push the tag.

Any static host works (it only has to serve files over HTTPS). `latest.json` should not be cached for long; a
few minutes is fine.

## How updates work

The app reads `https://devstack.holostack.in/latest.json`:

```json
{
  "version": "0.2.0",
  "url": "https://devstack.holostack.in/devstack_setup_0.2.0.exe",
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
- `go run ./tools/release check` confirms the key matches the app; `build-release.ps1` runs it first.
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
