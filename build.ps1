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

# Icon, manifest (per-monitor DPI) and version info for the app exe. A release
# version also goes into the exe's version info, which Setup and Windows show.
if ($Version -match '^\d+\.\d+\.\d+$') {
    Push-Location cmd\devstack-app
    go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc --product-version "$Version.0" --file-version "$Version.0"
    $code = $LASTEXITCODE
    Pop-Location
    if ($code) { throw "go-winres failed" }
} else {
    go generate ./cmd/devstack-app
    if ($LASTEXITCODE) { throw "go generate failed" }
}

go build -trimpath -ldflags $ldflags -o $OutDir\devstack.exe .\cmd\devstack
if ($LASTEXITCODE) { throw "build devstack.exe failed" }

# -H windowsgui: no console window when started from the Start menu or tray.
go build -trimpath -ldflags "$ldflags -H windowsgui" -o $OutDir\devstack-app.exe .\cmd\devstack-app
if ($LASTEXITCODE) { throw "build devstack-app.exe failed" }

Get-ChildItem $OutDir\devstack*.exe | Select-Object Name, @{n='MB';e={[math]::Round($_.Length/1MB,1)}}
