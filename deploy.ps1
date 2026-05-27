$ErrorActionPreference = "Stop"

$targetDir = "$env:USERPROFILE\.local\bin"
$outputPath = "yt-2-bili.exe"

Write-Host "Building yt-2-bili..."
go build -o $outputPath ./cmd/yt-2-bili

if (-not (Test-Path $outputPath)) {
    throw "Build failed: $outputPath not found"
}

Write-Host "Copying to $targetDir..."
New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
Copy-Item -Force $outputPath (Join-Path $targetDir $outputPath)

Write-Host "✅ Deployed: $targetDir\$outputPath"
