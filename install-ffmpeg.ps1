# 1. 统一代理配置
$proxyAddr = "http://127.0.0.1:7890"
$env:HTTP_PROXY = $env:HTTPS_PROXY = "http://127.0.0.1:7890"
[System.Net.WebRequest]::DefaultWebProxy = New-Object System.Net.WebProxy($proxyAddr)

# 2. 检查是否已安装
if (Get-Command "wt" -ErrorAction SilentlyContinue) {
    Write-Host "Windows Terminal already installed"; exit 0
}

if (-not (Get-Command "scoop" -ErrorAction SilentlyContinue)) {
    Write-Error "Scoop missing. Install it first: https://scoop.sh/"; exit 1
}

# 3. 添加 extras bucket 并安装
Write-Host "Installing Windows Terminal..."
scoop install ffmpeg yt-dlp

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Success"
} else {
    Write-Error "❌ Failed"
    exit 1
}
