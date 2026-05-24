param(
    [string]$Model = "Systran/faster-whisper-large-v3",
    [string]$LocalDir = "E:\Models\faster-whisper-large-v3",
    [string]$HfEndpoint = "https://hf-mirror.com",
    [int]$MaxWorkers = 8
)

$ErrorActionPreference = "Stop"
$env:HF_ENDPOINT = $HfEndpoint
$env:HF_HUB_DISABLE_TELEMETRY = "1"

$proxyVariables = @("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy")
foreach ($name in $proxyVariables) {
    [Environment]::SetEnvironmentVariable($name, $null, "Process")
}

if (-not (Get-Command hf -ErrorAction SilentlyContinue)) {
    Write-Host "Installing Hugging Face CLI..."
    uv tool install huggingface-hub
}

if (-not (Get-Command hf -ErrorAction SilentlyContinue)) {
    throw "Hugging Face CLI was not found after installation. Ensure uv tool bin is in PATH."
}

$requiredFiles = @(
    "config.json",
    "model.bin",
    "tokenizer.json",
    "vocabulary.json"
)

function Test-ModelInstalled {
    foreach ($file in $requiredFiles) {
        $path = Join-Path $LocalDir $file
        if (-not (Test-Path $path)) {
            return $false
        }
    }

    return $true
}

Write-Host "HF_ENDPOINT=$env:HF_ENDPOINT"
Write-Host "Proxy variables cleared for this process."
Write-Host "Preparing model directory: $LocalDir"
New-Item -ItemType Directory -Force -Path $LocalDir | Out-Null

if (Test-ModelInstalled) {
    Write-Host "✅ Faster Whisper model already exists: $LocalDir"
    Write-Host "💡 Use with: whisper-ctranslate2 <video> --model_directory `"$LocalDir`" --output_format srt --output_dir . --compute_type int8 --batched True --vad_filter True"
    exit 0
}

$lockFiles = Get-ChildItem -Path $LocalDir -Filter "*.lock" -Recurse -ErrorAction SilentlyContinue
foreach ($lockFile in $lockFiles) {
    Write-Host "Removing stale lock file: $($lockFile.FullName)"
    Remove-Item -Force $lockFile.FullName
}

Write-Host "⬇️ Downloading model: $Model"
Write-Host "Max workers: $MaxWorkers"
hf download $Model --local-dir $LocalDir --max-workers $MaxWorkers

if (-not (Test-ModelInstalled)) {
    foreach ($file in $requiredFiles) {
        $path = Join-Path $LocalDir $file
        if (-not (Test-Path $path)) {
            throw "Missing expected model file: $path"
        }
    }
}

Write-Host "✅ Faster Whisper model is ready: $LocalDir"
Write-Host "💡 Use with: whisper-ctranslate2 <video> --model_directory `"$LocalDir`" --output_format srt --output_dir . --compute_type int8 --batched True --vad_filter True"
