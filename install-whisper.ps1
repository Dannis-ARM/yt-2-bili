$ErrorActionPreference = "Stop"

# 🌐 Configure proxy for package installation, but bypass the Hugging Face mirror.
$proxyAddr = "http://127.0.0.1:7890"
$mirrorHost = "hf-mirror.com"

function Add-NoProxyHost {
    param(
        [string]$Name,
        [string]$HostName
    )

    $current = [Environment]::GetEnvironmentVariable($Name, "Process")
    if ([string]::IsNullOrWhiteSpace($current)) {
        [Environment]::SetEnvironmentVariable($Name, $HostName, "Process")
        return
    }

    $entries = $current -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    if ($entries -notcontains $HostName) {
        [Environment]::SetEnvironmentVariable($Name, ($entries + $HostName -join ","), "Process")
    }
}

$env:HTTP_PROXY = $proxyAddr
$env:HTTPS_PROXY = $proxyAddr
Add-NoProxyHost -Name "NO_PROXY" -HostName $mirrorHost
Add-NoProxyHost -Name "no_proxy" -HostName $mirrorHost

$proxy = New-Object System.Net.WebProxy($proxyAddr)
$proxy.BypassList = @($mirrorHost)
$proxy.BypassProxyOnLocal = $true
[System.Net.WebRequest]::DefaultWebProxy = $proxy

# 📦 Install OpenAI Whisper CLI.
uv tool install openai-whisper

# 📁 Prepare Whisper cache directory.
$cacheDir = Join-Path $HOME ".cache\whisper"
$modelPath = Join-Path $cacheDir "large-v3.pt"
$tmpModelPath = Join-Path $cacheDir "large-v3.pt.tmp"
New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null

# 🇨🇳 Download from Hugging Face mirror.
# ⚠️ Note: Hugging Face uses Transformers checkpoint format. If openai-whisper rejects this file,
# use `whisper --model turbo --model_dir $cacheDir ...` and let whisper download its native checkpoint.
$modelUrl = "https://hf-mirror.com/openai/whisper-large-v3-turbo/resolve/main/pytorch_model.bin?download=true"

if (Test-Path $modelPath) {
    $size = (Get-Item $modelPath).Length
    if ($size -gt 0) {
        Write-Host "✅ Model already exists: $modelPath ($size bytes)"
    } else {
        Write-Host "⚠️ Existing model is empty. Removing it..."
        Remove-Item -Force $modelPath
    }
}

if (-not (Test-Path $modelPath)) {
    if (Test-Path $tmpModelPath) {
        Write-Host "🧹 Removing previous temporary download: $tmpModelPath"
        Remove-Item -Force $tmpModelPath
    }

    Write-Host "⬇️ Downloading Whisper model to temporary file: $tmpModelPath"
    Invoke-WebRequest -Uri $modelUrl -OutFile $tmpModelPath -UseBasicParsing

    $tmpSize = (Get-Item $tmpModelPath).Length
    if ($tmpSize -le 0) {
        Remove-Item -Force $tmpModelPath
        throw "Downloaded model is empty: $tmpModelPath"
    }

    Write-Host "✅ Temporary download complete: $tmpModelPath ($tmpSize bytes)"
    Move-Item -Force $tmpModelPath $modelPath
    Write-Host "✅ Model installed: $modelPath"
}

whisper --help