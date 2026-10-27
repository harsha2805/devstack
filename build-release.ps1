<#
.SYNOPSIS
  Builds a devstack release into dist\release: the installer, a portable zip,
  SHA256SUMS.txt, and latest.json (the signed in-app update feed).

.DESCRIPTION
  Upload devstack_setup_<version>.exe and the zip to https://devstack.holostack.in/,
  then latest.json LAST, so the feed never points at an installer that isn't
  there yet. Previous release files are moved to dist\release\old\<version>.

  latest.json is signed with the offline release key (tools/release; see
  docs/RELEASING.md). The script checks the key before building and fails
  without it, because released apps ignore unsigned or wrongly signed feeds.

.EXAMPLE
  .\build-release.ps1 -Version 0.1.0 -Notes "First release."
#>
param(
    [Parameter(Mandatory)][string]$Version,
    [string]$Notes = "",
    [string]$BaseUrl = "https://devstack.holostack.in"
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Version must look like 1.2.3" }
Write-Host "Building devstack $Version" -ForegroundColor Cyan

# Fail fast if the release signing key is missing or not trusted by this build.
go run ./tools/release check
if ($LASTEXITCODE) { throw "Update signing key check failed (see docs/RELEASING.md)" }

$iscc = @("$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
          "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
          "$env:ProgramFiles\Inno Setup 6\ISCC.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $iscc) { throw "Inno Setup 6 not found. Install: winget install -e --id JRSoftware.InnoSetup --scope user" }

go test ./...
if ($LASTEXITCODE) { throw "Tests failed" }

$out = "dist\release"
New-Item -ItemType Directory -Force $out | Out-Null
# Release exes get their own folder, so a dev copy running from dist\ doesn't block the build.
$bin = "$out\bin"
if (Test-Path $bin) { Remove-Item $bin -Recurse -Force }
& .\build.ps1 -Version $Version -OutDir $bin
$previous = Get-ChildItem $out -File | Where-Object { $_.Name -match '^devstack_(setup_)?(\d+\.\d+\.\d+)' -and $Matches[2] -ne $Version }
foreach ($file in $previous) {
    $null = $file.Name -match '(\d+\.\d+\.\d+)'
    $archive = Join-Path $out "old\$($Matches[1])"
    New-Item -ItemType Directory -Force $archive | Out-Null
    Move-Item $file.FullName $archive -Force
    foreach ($meta in "SHA256SUMS.txt", "latest.json") {
        if (Test-Path "$out\$meta") { Move-Item "$out\$meta" (Join-Path $archive $meta) -Force }
    }
}

& $iscc /Q "/DAppVersion=$Version" "/DBin=..\$bin" installer\devstack.iss
if ($LASTEXITCODE) { throw "Inno Setup failed" }

$setup = "devstack_setup_$Version.exe"
$zip = "devstack_${Version}_portable.zip"
Compress-Archive -Path "$bin\devstack-app.exe", "$bin\devstack.exe" -DestinationPath "$out\$zip" -CompressionLevel Optimal -Force

$hashes = [ordered]@{}
foreach ($name in $setup, $zip) { $hashes[$name] = (Get-FileHash "$out\$name" -Algorithm SHA256).Hash.ToLower() }
($hashes.GetEnumerator() | ForEach-Object { "{0}  {1}" -f $_.Value, $_.Key }) -join "`n" |
    Set-Content "$out\SHA256SUMS.txt" -Encoding ascii -NoNewline

[ordered]@{
    version = $Version
    url     = "$BaseUrl/$setup"
    sha256  = $hashes[$setup]
    notes   = $Notes
    page    = $BaseUrl
} | ConvertTo-Json | Set-Content "$out\latest.json" -Encoding utf8
go run ./tools/release sign "$out\latest.json"
if ($LASTEXITCODE) { throw "Signing latest.json failed" }

Get-ChildItem $out -File | Select-Object Name, @{ n = 'MB'; e = { [math]::Round($_.Length / 1MB, 2) } } | Format-Table -AutoSize
Write-Host "Upload $setup and $zip, then latest.json, to $BaseUrl/" -ForegroundColor Green
