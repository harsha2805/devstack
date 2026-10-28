<#
.SYNOPSIS
  Releases devstack <version>: tags the pushed commit, waits for GitHub to
  build it, and makes sure it ends published, with the website told.

.DESCRIPTION
  Tags HEAD as v<version> and pushes the tag, which starts the release
  workflow (.github/workflows/release.yml): it builds and tests on GitHub's
  runners. With the DEVSTACK_UPDATE_KEY secret set, the workflow signs
  latest.json, publishes the release and tells the website itself; this
  script just waits and reports.

  Without that secret the workflow stops at a draft. This script then
  downloads the draft's files into dist\release, checks their hashes, writes
  latest.json, signs it with the release key on this machine, attaches it to
  the release, publishes it and tells the website.

  Running it again for a version that is building or published only reports.

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

# Errors such as "release not found" arrive on stderr, which Windows PowerShell
# turns into an exception under 'Stop'; ask through cmd and judge by exit code.
function Get-ReleaseState {
    $s = cmd /c "gh release view $tag --repo $ReleaseRepo --json isDraft --jq .isDraft 2>nul"
    if ($LASTEXITCODE) { return 'missing' }
    if ($s -eq 'true') { return 'draft' }
    return 'published'
}

$state = Get-ReleaseState
if ($state -eq 'missing') {
    # Release what is on GitHub, not local work in progress.
    if (git status --porcelain) { throw "Uncommitted changes. Commit (and push) them first." }
    git fetch -q --tags origin
    $head = git rev-parse HEAD
    if ($head -ne (git rev-parse origin/main)) { throw "HEAD isn't pushed to origin/main. Push first." }
    if (-not (git ls-remote --tags origin "refs/tags/$tag")) {
        if (-not (git tag -l $tag)) { git tag $tag $head }
        elseif ((git rev-list -n 1 $tag) -ne $head) { throw "A local tag $tag points elsewhere. Delete it (git tag -d $tag) and run this again." }
        cmd /c "git push -q origin $tag 2>&1"
        if ($LASTEXITCODE) { throw "Pushing $tag failed" }
        Write-Host "Pushed $tag; GitHub is building it." -ForegroundColor Green
    }
    Wait-ReleaseBuild $tag
    $state = Get-ReleaseState
    if ($state -eq 'missing') { throw "The build finished but left no release for $tag." }
}

if ($state -eq 'published') {
    Write-Host "Published https://github.com/$ReleaseRepo/releases/tag/$tag (signed and announced to the website by GitHub)." -ForegroundColor Green
    return
}

# A draft: the workflow had no signing key, so sign and publish from here.
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
cmd /c "gh release upload $tag $out\latest.json --repo $ReleaseRepo --clobber 2>&1"
if ($LASTEXITCODE) { throw "Attaching latest.json failed" }
cmd /c "gh release edit $tag --repo $ReleaseRepo --draft=false --latest 2>&1"
if ($LASTEXITCODE) { throw "Publishing the draft failed" }
Write-Host "Published https://github.com/$ReleaseRepo/releases/tag/$tag." -ForegroundColor Green
Send-ReleaseHook $Version
