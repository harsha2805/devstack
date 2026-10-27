<#
.SYNOPSIS
  Publishes a built release (build-release.ps1) as a GitHub release: tag
  v<version> on the pushed commit, with the installer, portable zip and
  SHA256SUMS.txt attached and the CHANGELOG.md section as its notes.

.DESCRIPTION
  Run website\deploy.ps1 afterwards: it includes latest.json in the site only
  once the installer URL below is live, so the in-app updater never sees a
  release before its files exist.

.EXAMPLE
  .\publish-release.ps1 -Version 0.1.0
#>
param(
    [Parameter(Mandatory)][string]$Version,
    [string]$Repo = "harsha2805/devstack"
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$out = "dist\release"
$setup = "$out\devstack_setup_$Version.exe"
$zip = "$out\devstack_${Version}_portable.zip"
foreach ($f in $setup, $zip, "$out\SHA256SUMS.txt", "$out\latest.json") {
    if (-not (Test-Path $f)) { throw "Missing $f. Run .\build-release.ps1 -Version $Version first." }
}

# The files must be the ones the signed feed describes.
$feed = Get-Content "$out\latest.json" -Raw | ConvertFrom-Json
if ($feed.version -ne $Version) { throw "latest.json is for $($feed.version), not $Version." }
$hash = (Get-FileHash $setup -Algorithm SHA256).Hash.ToLower()
if ($hash -ne $feed.sha256.ToLower()) { throw "The installer doesn't match latest.json's sha256. Rebuild with build-release.ps1." }
if ($feed.url -notlike "*/releases/download/v$Version/devstack_setup_$Version.exe") { throw "latest.json points at $($feed.url), not this GitHub release." }

# Publish what is on GitHub, not local work in progress.
if (git status --porcelain) { throw "Uncommitted changes. Commit (and push) them first." }
git fetch -q origin
$head = git rev-parse HEAD
if ($head -ne (git rev-parse origin/main)) { throw "HEAD isn't pushed to origin/main. Push first." }
# "release not found" arrives on stderr, which Windows PowerShell turns into an
# error under 'Stop'; ask through cmd and judge by the exit code alone.
cmd /c "gh release view v$Version --repo $Repo >nul 2>&1"
if ($LASTEXITCODE -eq 0) { throw "Release v$Version already exists on GitHub." }

$changelog = Get-Content CHANGELOG.md -Raw
$section = [regex]::Match($changelog, "(?ms)^## \[$([regex]::Escape($Version))\][^\r\n]*\r?\n(.*?)(?=^## \[|\z)")
if (-not $section.Success) { throw "CHANGELOG.md has no '## [$Version]' section." }
$notes = Join-Path ([IO.Path]::GetTempPath()) "devstack-notes-$Version.md"
@"
$($section.Groups[1].Value.Trim())

---

**Install:** download ``devstack_setup_$Version.exe`` below and run it. It installs for your account, with no administrator prompt.
**Verify:** the SHA-256 of each file is in ``SHA256SUMS.txt``.
"@ | Set-Content $notes -Encoding utf8

cmd /c "gh release create v$Version `"$setup`" `"$zip`" `"$out\SHA256SUMS.txt`" --repo $Repo --target $head --title `"devstack $Version`" --notes-file `"$notes`" 2>&1"
if ($LASTEXITCODE) { throw "gh release create failed" }
Remove-Item $notes
Write-Host "Published https://github.com/$Repo/releases/tag/v$Version. Next: website\deploy.ps1" -ForegroundColor Green
