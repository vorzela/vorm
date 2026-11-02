# vorm per-user installer (no Administrator required).
# Usage: iex (New-Object Net.WebClient).DownloadString('https://raw.githubusercontent.com/vorzela/vorm/main/install.ps1')
$ErrorActionPreference = "Stop"

function Info($m) { Write-Host "ℹ $m" -ForegroundColor Cyan }
function Ok($m) { Write-Host "✓ $m" -ForegroundColor Green }
function Warn($m) { Write-Host "⚠ $m" -ForegroundColor Yellow }
function Fail($m) { Write-Host "✗ $m" -ForegroundColor Red; exit 1 }

$InstallDir = if ($env:VORM_INSTALL_DIR) { $env:VORM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\vorm\bin" }
$Version = if ($env:VORM_VERSION) { $env:VORM_VERSION } else { "latest" }
$Binary = Join-Path $InstallDir "vorm.exe"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail "Go is required. Install Go from https://go.dev/dl/ (no Administrator needed)."
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Info "Installing vorm@$Version into $InstallDir (per-user; no Administrator)"
$env:GOBIN = $InstallDir
go install "github.com/vorzela/vorm/cmd/vorm@$Version"
if (-not (Test-Path $Binary)) {
    $candidate = Join-Path $InstallDir "vorm.exe"
    if (Test-Path $candidate) { $Binary = $candidate } else { Fail "go install did not produce $Binary" }
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$userPath", "User")
    $env:Path = "$InstallDir;$env:Path"
    Info "Added $InstallDir to your user PATH"
}

Ok "Installed $(& $Binary version)"
Info "Binary: $Binary"
Info "Restart PowerShell to refresh PATH."
