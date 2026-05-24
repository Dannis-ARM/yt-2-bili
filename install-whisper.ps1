$ErrorActionPreference = "Stop"

# 🌐 Configure proxy for Python package installation.
$proxyAddr = "http://127.0.0.1:7890"
$env:HTTP_PROXY = $proxyAddr
$env:HTTPS_PROXY = $proxyAddr
[System.Net.WebRequest]::DefaultWebProxy = New-Object System.Net.WebProxy($proxyAddr)

# 📦 Install OpenAI Whisper CLI.
uv tool install openai-whisper

# TODO try on whisper.cpp