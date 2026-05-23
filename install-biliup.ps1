# Install biliup-rs for current user
# Downloads and installs biliup-rs v1.1.29 and adds to PATH using winpath

param(
    [switch]$Force
)

$ErrorActionPreference = "Stop"

$version = "1.1.29"
$url = "https://github.com/biliup/biliup/releases/download/v$version/biliupR-v$version-x86_64-windows.zip"
$installDir = "$env:LOCALAPPDATA\biliup"
$zipPath = "$env:TEMP\biliupR-v$version-x86_64-windows.zip"

Write-Host "🍃 Installing biliup-rs v$version..." -ForegroundColor Cyan

# Check for winpath first
Write-Host "🔍 Checking for winpath..."
if (-not (Get-Command "winpath" -ErrorAction SilentlyContinue)) {
    Write-Error "❌ winpath not found! Please install winpath first."
    exit 1
}
Write-Host "✅ winpath found"

# Create install directory
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Write-Host "📁 Created directory: $installDir"
}

# Download zip if not exists or -Force is specified
if ((Test-Path $zipPath) -and -not $Force) {
    Write-Host "ℹ️ Using existing zip file: $zipPath"
    Write-Host "   Use -Force to re-download"
} else {
    Write-Host "⬇️ Downloading biliup-rs..."
    $webClient = New-Object System.Net.WebClient
    $webClient.DownloadFile($url, $zipPath)
    Write-Host "✅ Downloaded to: $zipPath"
}

# Extract zip
Write-Host "📦 Extracting..."
Expand-Archive -Path $zipPath -DestinationPath $installDir -Force
Write-Host "✅ Extracted to: $installDir"

# Cleanup zip
Remove-Item $zipPath -Force
Write-Host "🗑️ Cleaned up zip file"

# Find biliup.exe
$biliupExe = Get-ChildItem -Path $installDir -Recurse -Filter "biliup.exe" | Select-Object -First 1 -ExpandProperty FullName

if (-not $biliupExe) {
    Write-Error "❌ biliup.exe not found in extracted files!"
    exit 1
}

$biliupDir = Split-Path -Parent $biliupExe
Write-Host "📍 Found biliup.exe at: $biliupDir"

# Add to PATH using winpath
Write-Host "🔧 Adding to PATH using winpath..."
& winpath add $biliupDir
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Added to PATH successfully!" -ForegroundColor Green
} else {
    Write-Error "❌ winpath exited with code $LASTEXITCODE"
    exit 1
}

Write-Host "`n🎉 Installation complete!" -ForegroundColor Green
Write-Host "👉 To use, restart your terminal or refresh PATH, then run: biliup --help"
