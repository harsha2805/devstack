<#
.SYNOPSIS
  Packages the website for shared hosting: ..\dist\website.zip, ready to upload
  and extract (see ..\docs\WEBSITE.md).

.DESCRIPTION
  - Copies ..\CHANGELOG.md into resources\markdown\.
  - Copies ..\dist\release\latest.json into public\ once its download URL is live
    on GitHub Releases, so the site never links to a release that isn't there.
  - Installs production dependencies (no dev packages) into a staging folder,
    because shared hosting usually has no Composer.
  - Uses .env.production as the server's .env, creating it with a fresh APP_KEY
    the first time. It is gitignored: keep it, the key must stay the same.

.EXAMPLE
  .\deploy.ps1
#>
param([string]$Out = "..\dist\website.zip")

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

New-Item -ItemType Directory -Force resources\markdown | Out-Null
Copy-Item ..\CHANGELOG.md resources\markdown\CHANGELOG.md -Force

$feed = "..\dist\release\latest.json"
if (Test-Path $feed) {
    $url = (Get-Content $feed -Raw | ConvertFrom-Json).url
    try {
        Invoke-WebRequest $url -Method Head -UseBasicParsing -MaximumRedirection 5 | Out-Null
        Copy-Item $feed public\latest.json -Force
        Write-Host "latest.json: $url is live, included." -ForegroundColor Green
    } catch {
        Write-Warning "latest.json not included: $url isn't reachable yet. Publish the GitHub release first (..\publish-release.ps1)."
    }
} else {
    Write-Warning "No ..\dist\release\latest.json; the site shows 'coming soon' until a release is built and published."
}

if (-not (Test-Path .env.production)) {
    $key = (php artisan key:generate --show).Trim()
    (Get-Content .env.example) `
        -replace '^APP_ENV=.*', 'APP_ENV=production' `
        -replace '^APP_DEBUG=.*', 'APP_DEBUG=false' `
        -replace '^APP_KEY=.*', "APP_KEY=$key" `
        -replace '^LOG_LEVEL=.*', 'LOG_LEVEL=error' |
        Set-Content .env.production -Encoding ascii
    Write-Host "Created .env.production with a new APP_KEY. Keep this file; it isn't in git." -ForegroundColor Yellow
}

$stage = Join-Path ([IO.Path]::GetTempPath()) "devstack-website-$([guid]::NewGuid().ToString('n'))"
New-Item -ItemType Directory $stage | Out-Null
try {
    # robocopy exit codes below 8 mean success.
    robocopy . $stage /E /NFL /NDL /NJH /NJS /NP `
        /XD vendor node_modules tests .git .idea .vscode `
        /XF .env .env.* *.log database.sqlite .phpunit.result.cache deploy.ps1 | Out-Null
    if ($LASTEXITCODE -ge 8) { throw "robocopy failed ($LASTEXITCODE)" }
    Copy-Item .env.production (Join-Path $stage .env)
    foreach ($d in "storage\framework\cache\data", "storage\framework\sessions", "storage\framework\views", "storage\logs") {
        Get-ChildItem (Join-Path $stage $d) -File -ErrorAction SilentlyContinue | Where-Object Name -ne .gitignore | Remove-Item
    }
    # Composer reports progress on stderr, which Windows PowerShell turns into an
    # error under 'Stop'; merge it inside cmd and judge by the exit code.
    cmd /c "composer install --no-dev --optimize-autoloader --no-interaction --no-progress --working-dir `"$stage`" 2>&1" | Select-Object -Last 2
    if ($LASTEXITCODE) { throw "composer install failed" }

    $zip = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot $Out))
    New-Item -ItemType Directory -Force (Split-Path $zip) | Out-Null
    if (Test-Path $zip) { Remove-Item $zip }
    # Not Compress-Archive: Windows PowerShell writes "public\index.php", which a
    # Linux host's unzip extracts as one oddly named file instead of folders.
    Add-Type -AssemblyName System.IO.Compression, System.IO.Compression.FileSystem
    $archive = [IO.Compression.ZipFile]::Open($zip, 'Create')
    try {
        $base = (Resolve-Path $stage).Path.TrimEnd('\') + '\'
        foreach ($file in Get-ChildItem $stage -Recurse -File -Force) {
            $name = $file.FullName.Substring($base.Length).Replace('\', '/')
            [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $file.FullName, $name, 'Optimal') | Out-Null
        }
    } finally { $archive.Dispose() }
    Write-Host ("{0} ({1:N1} MB). Upload and extract it as described in docs\WEBSITE.md." -f $zip, ((Get-Item $zip).Length / 1MB)) -ForegroundColor Green
} finally {
    Remove-Item $stage -Recurse -Force -ErrorAction SilentlyContinue
}
