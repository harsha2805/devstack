<#
.SYNOPSIS
  Releases devstack <version> in two runs: the first tags the pushed commit,
  which starts the GitHub build; the second publishes that build.

.DESCRIPTION
  Run 1: tags HEAD as v<version> and pushes the tag. The release workflow
  (.github/workflows/release.yml) builds and tests on GitHub's runners and
  leaves a draft release with the installer, portable zip and SHA256SUMS.txt.

  Run 2, once the draft exists: downloads its files into dist\release, checks
  their hashes, writes latest.json and signs it with the offline release key,
  and publishes the draft. Then run website\deploy.ps1.

.EXAMPLE
  .\publish-release.ps1 -Version 0.2.0
#>
param(
    [Parameter(Mandatory)][string]$Version,
    [string]$Notes = ""
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
. .\release-lib.ps1

if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Version must look like 1.2.3" }
$tag = "v$Version"
if (-not $Notes) { $Notes = Get-UpdateNotes $Version }

# Release what is on GitHub, not local work in progress.
if (git status --porcelain) { throw "Uncommitted changes. Commit (and push) them first." }
git fetch -q --tags origin
$head = git rev-parse HEAD
if ($head -ne (git rev-parse origin/main)) { throw "HEAD isn't pushed to origin/main. Push first." }

# Errors such as "release not found" arrive on stderr, which Windows PowerShell
# turns into an exception under 'Stop'; ask through cmd and judge by exit code.
$state = cmd /c "gh release view $tag --repo $ReleaseRepo --json isDraft --jq .isDraft 2>nul"
if ($LASTEXITCODE) {
    if (git ls-remote --tags origin "refs/tags/$tag") {
        Write-Host "$tag is pushed but its draft release isn't there yet. Watch the build:" -ForegroundColor Yellow
        Write-Host "  https://github.com/$ReleaseRepo/actions/workflows/release.yml"
        Write-Host "Once it finishes, run this again."
        return
    }
    if (-not (git tag -l $tag)) { git tag $tag $head }
    elseif ((git rev-list -n 1 $tag) -ne $head) { throw "A local tag $tag points elsewhere. Delete it (git tag -d $tag) and run this again." }
    cmd /c "git push -q origin $tag 2>&1"
    if ($LASTEXITCODE) { throw "Pushing $tag failed" }
    Write-Host "Pushed $tag; GitHub is building it: https://github.com/$ReleaseRepo/actions/workflows/release.yml" -ForegroundColor Green
    Write-Host "Once the draft release appears (a few minutes), run .\publish-release.ps1 -Version $Version again."
    return
}
if ($state -ne 'true') { throw "Release $tag is already published." }
if ((git rev-list -n 1 $tag) -ne $head) { throw "$tag points at $(git rev-list -n 1 $tag), not HEAD ($head)." }

$out = "dist\release"
New-Item -ItemType Directory -Force $out | Out-Null
Move-OldReleaseFiles $out $Version
$setup = "devstack_setup_$Version.exe"
$zip = "devstack_${Version}_portable.zip"
foreach ($f in $setup, $zip, "SHA256SUMS.txt", "latest.json") { Remove-Item "$out\$f" -ErrorAction SilentlyContinue }
cmd /c "gh release download $tag --repo $ReleaseRepo --dir $out --pattern $setup --pattern $zip --pattern SHA256SUMS.txt 2>&1"
if ($LASTEXITCODE) { throw "Downloading the draft's files failed" }

# The files must be the ones the build listed.
foreach ($line in Get-Content "$out\SHA256SUMS.txt") {
    $hash, $name = $line -split '\s+', 2
    if ((Get-FileHash "$out\$name" -Algorithm SHA256).Hash.ToLower() -ne $hash) { throw "$name doesn't match SHA256SUMS.txt" }
}

Write-UpdateFeed $Version $out $Notes
cmd /c "gh release edit $tag --repo $ReleaseRepo --draft=false --latest 2>&1"
if ($LASTEXITCODE) { throw "Publishing the draft failed" }
Write-Host "Published https://github.com/$ReleaseRepo/releases/tag/$tag. Next: website\deploy.ps1" -ForegroundColor Green
