<#
.SYNOPSIS
  Builds devstack.exe (CLI) and devstack-app.exe (tray + window) into dist\
  (or -OutDir).

.EXAMPLE
  .\build.ps1 -Version 0.1.0
#>
param([string]$Version = "dev", [string]$OutDir = "dist")

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$ldflags = "-s -w -X main.version=$Version"

# Icon, manifest and version info (publisher, copyright) for both exes. A
# release version also goes into it, which Setup and Windows show.
foreach ($cmd in "devstack-app", "devstack") {
    Push-Location "cmd\$cmd"
    if ($Version -match '^\d+\.\d+\.\d+$') {
        go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc --product-version "$Version" --file-version "$Version"
    } else {
        go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc
    }
    $code = $LASTEXITCODE
    Pop-Location
    if ($code) { throw "go-winres failed for $cmd" }
}

go build -trimpath -ldflags $ldflags -o $OutDir\devstack.exe .\cmd\devstack
if ($LASTEXITCODE) { throw "build devstack.exe failed" }

# -H windowsgui: no console window when started from the Start menu or tray.
go build -trimpath -ldflags "$ldflags -H windowsgui" -o $OutDir\devstack-app.exe .\cmd\devstack-app
if ($LASTEXITCODE) { throw "build devstack-app.exe failed" }

# The php/composer shim, twice under the names it answers to (package shim).
New-Item -ItemType Directory -Force $OutDir\shims | Out-Null
go build -trimpath -ldflags $ldflags -o $OutDir\shims\php.exe .\cmd\php-shim
if ($LASTEXITCODE) { throw "build php-shim failed" }
Copy-Item $OutDir\shims\php.exe $OutDir\shims\composer.exe -Force

Get-ChildItem $OutDir\devstack*.exe | Select-Object Name, @{n='MB';e={[math]::Round($_.Length/1MB,1)}}
