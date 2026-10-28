<#
.SYNOPSIS
  Builds a devstack release locally into dist\release: the installer, a
  portable zip and SHA256SUMS.txt, to try out before tagging.

.DESCRIPTION
  This build is unsigned and is never published. Released files come from the
  GitHub workflow (.github/workflows/release.yml), which builds the same way on
  GitHub's runners; publish-release.ps1 starts it and then publishes its draft.
  See docs/RELEASING.md.

  Previous release files are moved to dist\release\old\<version>.

.EXAMPLE
  .\build-release.ps1 -Version 0.2.0
  Needs a "## [0.2.0]" section in CHANGELOG.md.
#>
param([Parameter(Mandatory)][string]$Version)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
. .\release-lib.ps1

if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Version must look like 1.2.3" }
Write-Host "Building devstack $Version" -ForegroundColor Cyan

# Every release is written up in CHANGELOG.md first; fail now rather than at publish time.
$null = Get-UpdateNotes $Version
$null = Find-Iscc

go test ./...
if ($LASTEXITCODE) { throw "Tests failed" }

$out = "dist\release"
New-Item -ItemType Directory -Force $out | Out-Null
# Release exes get their own folder, so a dev copy running from dist\ doesn't block the build.
$bin = "$out\bin"
if (Test-Path $bin) { Remove-Item $bin -Recurse -Force }
& .\build.ps1 -Version $Version -OutDir $bin
Move-OldReleaseFiles $out $Version
New-Installer $Version $bin
New-ReleaseFiles $Version $bin $out

Get-ChildItem $out -File | Select-Object Name, @{ n = 'MB'; e = { [math]::Round($_.Length / 1MB, 2) } } | Format-Table -AutoSize
Write-Host "To release it: commit, push, then .\publish-release.ps1 -Version $Version" -ForegroundColor Green
