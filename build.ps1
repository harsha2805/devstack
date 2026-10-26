<#
.SYNOPSIS
  Builds devstack.exe (CLI) and devstack-app.exe (tray + window) into dist\.

.EXAMPLE
  .\build.ps1 -Version 0.1.0
#>
param([string]$Version = "dev")

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$ldflags = "-s -w -X main.version=$Version"

# Icon, manifest (per-monitor DPI) and version info for the app exe.
go generate ./cmd/devstack-app
if ($LASTEXITCODE) { throw "go generate failed" }

go build -trimpath -ldflags $ldflags -o dist\devstack.exe .\cmd\devstack
if ($LASTEXITCODE) { throw "build devstack.exe failed" }

# -H windowsgui: no console window when started from the Start menu or tray.
go build -trimpath -ldflags "$ldflags -H windowsgui" -o dist\devstack-app.exe .\cmd\devstack-app
if ($LASTEXITCODE) { throw "build devstack-app.exe failed" }

Get-ChildItem dist\devstack*.exe | Select-Object Name, @{n='MB';e={[math]::Round($_.Length/1MB,1)}}
