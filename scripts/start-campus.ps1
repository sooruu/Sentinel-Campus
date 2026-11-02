[CmdletBinding()]
param(
    [switch]$SkipInstall,
    [switch]$AutoGate,
    [ValidateSet('simulator', 'webhook')]
    [string]$Hardware = 'simulator',
    [int]$Port = 8088
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Require-Command([string]$Name, [string]$InstallId, [string]$InstallName) {
    if (Get-Command $Name -ErrorAction SilentlyContinue) { return }
    if ($SkipInstall) {
        throw "$InstallName is required. Install it or run this script without -SkipInstall."
    }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        throw "$InstallName is missing and winget is unavailable. Install $InstallName from https://go.dev/dl/ or https://nodejs.org/ and run this script again."
    }
    Write-Host "Installing $InstallName..." -ForegroundColor Cyan
    winget install --id $InstallId --exact --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) {
        throw "winget could not install $InstallName (exit code $LASTEXITCODE)."
    }
    # winget updates the machine PATH, but the current PowerShell process may
    # still have the old value. Reload it before the next prerequisite check.
    $machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:Path = "$machinePath;$userPath"
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$InstallName was installed, but '$Name' is not available in this PowerShell session. Close PowerShell, open it again, and rerun the script."
    }
}

Write-Host 'Sentinel Campus Windows launcher' -ForegroundColor White
Write-Host "Project: $root"

Require-Command 'go' 'GoLang.Go' 'Go'
Require-Command 'node' 'OpenJS.NodeJS.LTS' 'Node.js LTS'
Require-Command 'npm' 'OpenJS.NodeJS.LTS' 'npm'

if (-not $SkipInstall) {
    Write-Host 'Installing Campus frontend dependencies...' -ForegroundColor Cyan
    Push-Location (Join-Path $root 'web-campus')
    try { npm ci } finally { Pop-Location }
}

Write-Host 'Building Campus frontend...' -ForegroundColor Cyan
Push-Location (Join-Path $root 'web-campus')
try { npm run build } finally { Pop-Location }

$env:SENTINEL_CAMPUS_HARDWARE = $Hardware
$env:SENTINEL_CAMPUS_PORT = [string]$Port
$env:SENTINEL_CAMPUS_EVENTS_FILE = Join-Path $root '.sentinel-campus-events.json'
$env:SENTINEL_CAMPUS_REGISTRY_FILE = Join-Path $root '.sentinel-campus-registry.json'
if (-not $env:SENTINEL_CAMPUS_LIVE_HLS_BASE) {
    $env:SENTINEL_CAMPUS_LIVE_HLS_BASE = 'http://127.0.0.1:18888'
}
if ($AutoGate) { $env:SENTINEL_CAMPUS_AUTO_GATE = 'gate-1' }

Write-Host ''
Write-Host "Sentinel Campus: http://localhost:$Port" -ForegroundColor Green
Write-Host "Hardware backend: $Hardware"
Write-Host 'All gates start in SHADOW mode unless -AutoGate is supplied.'
Write-Host 'Press Ctrl+C to stop the server.'
Write-Host ''

go run ./cmd/campus-demo
