# ============================================================
#  CEREBRO - one-line installer (Windows / PowerShell)
#
#  Usage (run in PowerShell):
#    irm https://raw.githubusercontent.com/NamanGaonkar/CEREBRO/main/install.ps1 | iex
#
#  Downloads the latest prebuilt binary from GitHub Releases.
#  If no release exists yet, falls back to building from source
#  (requires Git + Go). Installs to ~/.cerebro/bin and adds it
#  to your PATH. No admin rights required.
# ============================================================

$Owner   = "NamanGaonkar"
$Repo    = "CEREBRO"
$Version = "latest"   # or pin e.g. "v1.0.0"

$ErrorActionPreference = "Stop"

$InstallDir = Join-Path $HOME ".cerebro\bin"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# --- detect architecture ---
$Arch = $env:PROCESSOR_ARCHITECTURE
if ($Arch -eq "ARM64") { $Arch = "arm64" } else { $Arch = "amd64" }

# --- try downloading a prebuilt release ---
$Downloaded = $false
try {
    if ($Version -eq "latest") {
        Write-Host "Checking latest CEREBRO release ..."
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Owner/$Repo/releases/latest" -Headers @{ "User-Agent" = "cerebro-installer" }
        $Version = $release.tag_name
    }
    $Asset = "cerebro-windows-$Arch.zip"
    $Url   = "https://github.com/$Owner/$Repo/releases/download/$Version/$Asset"
    $Tmp   = Join-Path $env:TEMP "cerebro-$Version.zip"
    Write-Host "Downloading CEREBRO $Version ($Arch) from GitHub Releases ..."
    Invoke-WebRequest -Uri $Url -OutFile $Tmp
    Expand-Archive -Path $Tmp -DestinationPath $InstallDir -Force
    Remove-Item $Tmp -Force
    $Downloaded = $true
} catch {
    Write-Host "No prebuilt release found yet." -ForegroundColor Yellow
    $Downloaded = $false
}

# --- fallback: build from source ---
if (-not $Downloaded) {
    $Git = Get-Command git -ErrorAction SilentlyContinue
    $Go  = Get-Command go  -ErrorAction SilentlyContinue
    if (-not $Git -or -not $Go) {
        Write-Host ""
        Write-Host "[ERROR] No GitHub Release for CEREBRO yet AND Git/Go are not installed." -ForegroundColor Red
        Write-Host "        Either create a release (git tag v1.0.0; git push origin v1.0.0)" -ForegroundColor Red
        Write-Host "        or install Git and Go (winget install Git.Git GoLang.Go) and retry." -ForegroundColor Red
        exit 1
    }
    Write-Host "Building CEREBRO from source ..."
    $Src = Join-Path $env:TEMP ("cerebro-src-" + [guid]::NewGuid().ToString("N"))
    git clone --depth 1 "https://github.com/$Owner/$Repo.git" $Src
    Push-Location $Src
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags "-s -w" -o "$InstallDir\cerebro.exe" ./cmd/app
    if ($LASTEXITCODE -ne 0) {
        Pop-Location
        Write-Host "[ERROR] Build failed." -ForegroundColor Red
        exit 1
    }
    Pop-Location
    Remove-Item -Recurse -Force $Src
}

# --- verify the binary runs ---
$Exe = Join-Path $InstallDir "cerebro.exe"
if (-not (Test-Path $Exe)) {
    Write-Host "[ERROR] Binary missing at $Exe" -ForegroundColor Red
    exit 1
}
$Out = & $Exe --version
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Binary failed to run: $Out" -ForegroundColor Red
    exit 1
}

# --- add to user PATH (persists, no admin needed) ---
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
}
# also refresh PATH in THIS window so cerebro works immediately
$env:Path = "$env:Path;$InstallDir"

Write-Host ""
Write-Host "  $Out" -ForegroundColor Green
Write-Host "  CEREBRO installed to: $InstallDir"
Write-Host "  Run:  cerebro" -ForegroundColor Cyan
Write-Host "  (also added permanently to your PATH for all new terminals)" -ForegroundColor Yellow
Write-Host "  Built by Naman Gaonkar - https://naman-gaonkar.vercel.app/" -ForegroundColor Magenta
