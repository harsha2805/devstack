<#
.SYNOPSIS
  Shared release steps, dot-sourced by build-release.ps1, publish-release.ps1
  and the GitHub release workflow (.github/workflows/release.yml), so a local
  build and the CI build package the same way.
#>

$ReleaseRepo = "harsha2805/devstack"
$ReleaseSite = "https://devstack.holostack.in"

# The "## [x.y.z]" section of CHANGELOG.md, without its heading.
function Get-ChangelogSection([string]$Version) {
    $changelog = Get-Content (Join-Path $PSScriptRoot CHANGELOG.md) -Raw
    $section = [regex]::Match($changelog, "(?ms)^## \[$([regex]::Escape($Version))\][^\r\n]*\r?\n(.*?)(?=^## \[|\z)")
    if (-not $section.Success) { throw "CHANGELOG.md has no '## [$Version]' section. Write up this release's changes first." }
    $section.Groups[1].Value.Trim()
}

# The notes shown in Settings > Updates: the section's bold lead-ins, as plain text.
function Get-UpdateNotes([string]$Version) {
    $notes = ([regex]::Matches((Get-ChangelogSection $Version), '(?m)^- \*\*(.+?)\*\*') | ForEach-Object { $_.Groups[1].Value }) -join "`n"
    $notes = $notes.Replace([string][char]96, '')   # the app shows plain text, not Markdown code spans
    if (-not $notes) { throw "The CHANGELOG.md section for $Version has no '- **Lead-in.**' items to use as update notes; pass -Notes." }
    $notes
}

# The GitHub release's description.
function Get-ReleaseBody([string]$Version) {
    @"
$(Get-ChangelogSection $Version)

---

**Install:** download ``devstack_setup_$Version.exe`` below and run it. It installs for your account, with no administrator prompt.
**Verify:** the SHA-256 of each file is in ``SHA256SUMS.txt``.
**Not code-signed:** if Windows says "Windows protected your PC", choose More info, then Run anyway.
"@
}

function Find-Iscc {
    $iscc = @("$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
              "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
              "$env:ProgramFiles\Inno Setup 6\ISCC.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $iscc) { throw "Inno Setup 6 not found. Install: winget install -e --id JRSoftware.InnoSetup --scope user" }
    $iscc
}

# Builds dist\release\devstack_setup_<version>.exe from the exes in $Bin.
function New-Installer([string]$Version, [string]$Bin) {
    $bin = (Resolve-Path $Bin).Path
    & (Find-Iscc) /Q "/DAppVersion=$Version" "/DBin=$bin" (Join-Path $PSScriptRoot installer\devstack.iss)
    if ($LASTEXITCODE) { throw "Inno Setup failed" }
}

# Moves files of other versions out of $Out into $Out\old\<version>.
function Move-OldReleaseFiles([string]$Out, [string]$Version) {
    $previous = Get-ChildItem $Out -File | Where-Object { $_.Name -match '^devstack_(setup_)?(\d+\.\d+\.\d+)' -and $Matches[2] -ne $Version }
    foreach ($file in $previous) {
        $null = $file.Name -match '(\d+\.\d+\.\d+)'
        $archive = Join-Path $Out "old\$($Matches[1])"
        New-Item -ItemType Directory -Force $archive | Out-Null
        Move-Item $file.FullName $archive -Force
        foreach ($meta in "SHA256SUMS.txt", "latest.json") {
            if (Test-Path "$Out\$meta") { Move-Item "$Out\$meta" (Join-Path $archive $meta) -Force }
        }
    }
}

# Writes the portable zip and SHA256SUMS.txt next to the installer.
function New-ReleaseFiles([string]$Version, [string]$Bin, [string]$Out) {
    $setup = "devstack_setup_$Version.exe"
    $zip = "devstack_${Version}_portable.zip"
    Compress-Archive -Path "$Bin\devstack-app.exe", "$Bin\devstack.exe", "$Bin\shims" -DestinationPath "$Out\$zip" -CompressionLevel Optimal -Force
    $lines = foreach ($name in $setup, $zip) { "{0}  {1}" -f (Get-FileHash "$Out\$name" -Algorithm SHA256).Hash.ToLower(), $name }
    $lines -join "`n" | Set-Content "$Out\SHA256SUMS.txt" -Encoding ascii -NoNewline
}

# Writes latest.json for the installer in $Out and signs it with the offline
# release key. Released apps ignore an unsigned or wrongly signed feed.
function Write-UpdateFeed([string]$Version, [string]$Out, [string]$Notes) {
    $setup = "devstack_setup_$Version.exe"
    [ordered]@{
        version = $Version
        url     = "https://github.com/$ReleaseRepo/releases/download/v$Version/$setup"
        sha256  = (Get-FileHash "$Out\$setup" -Algorithm SHA256).Hash.ToLower()
        notes   = $Notes
        page    = $ReleaseSite
    } | ConvertTo-Json | Set-Content "$Out\latest.json" -Encoding utf8
    Push-Location $PSScriptRoot
    go run ./tools/release sign "$Out\latest.json"
    $code = $LASTEXITCODE
    Pop-Location
    if ($code) { throw "Signing latest.json failed" }
}

# Throws unless every file carries a valid Authenticode signature.

# Waits for the release workflow run of $Tag to finish and throws if it failed.
function Wait-ReleaseBuild([string]$Tag) {
    $id = $null
    foreach ($i in 1..30) {
        $id = cmd /c "gh run list --repo $ReleaseRepo --workflow release.yml --branch $Tag --limit 1 --json databaseId --jq .[0].databaseId 2>nul"
        if ($id) { break }
        Start-Sleep -Seconds 2
    }
    if (-not $id) { throw "GitHub hasn't started a build for $Tag. See https://github.com/$ReleaseRepo/actions/workflows/release.yml" }
    Write-Host "Waiting for the build: https://github.com/$ReleaseRepo/actions/runs/$id"
    cmd /c "gh run watch $id --repo $ReleaseRepo --exit-status --interval 15 >nul 2>&1"
    if ($LASTEXITCODE) { throw "The build of $Tag failed: https://github.com/$ReleaseRepo/actions/runs/$id" }
}

# Tells the website a release is out (POST /hooks/release), so it fetches the
# new feed and changelog from GitHub now rather than on its daily refresh. The
# token is RELEASE_HOOK_TOKEN from the environment (GitHub's secret) or from
# website\.env.production. Never fails the release: the site catches up anyway.
function Send-ReleaseHook([string]$Version) {
    $token = $env:RELEASE_HOOK_TOKEN
    $envFile = Join-Path $PSScriptRoot "website\.env.production"
    if (-not $token -and (Test-Path $envFile)) {
        $line = Get-Content $envFile | Where-Object { $_ -match '^RELEASE_HOOK_TOKEN=(.+)$' } | Select-Object -First 1
        if ($line) { $token = ($line -replace '^RELEASE_HOOK_TOKEN=', '').Trim().Trim('"') }
    }
    # A token pasted or piped through Windows PowerShell can pick up a BOM or a line
    # break, and HTTP headers take printable ASCII only.
    $token = $token -replace '[^\x21-\x7E]', ''
    if (-not $token) { Write-Warning "No RELEASE_HOOK_TOKEN: the website shows $Version within a day."; return }
    foreach ($try in 1..5) {
        try {
            $r = Invoke-RestMethod -Method Post "$ReleaseSite/hooks/release" -Headers @{ Authorization = "Bearer $token" } -TimeoutSec 30
            if ($r.version -eq $Version) { Write-Host "The website now offers $Version." -ForegroundColor Green; return }
            Write-Host "The website still sees $($r.version); GitHub may not list $Version yet."
        } catch {
            Write-Host "Telling the website, try ${try}: $($_.Exception.Message)"
        }
        Start-Sleep -Seconds (10 * $try)
    }
    Write-Warning "The website didn't confirm $Version. It picks it up within a day."
}
