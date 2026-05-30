# yt-2-bili 测试手册

> 每个 stage 独立可运行，复制命令即可测试。

---

## TL;DR — 完整流水线（最常用）

一键完成：YouTube 下载 → 字幕生成 → 中文翻译 → Bilibili 上传。

```powershell
# === 前置准备 ===
$TEST_URL = "https://www.youtube.com/watch?v=mGEfasQl2Zo"
$COOKIE = "$env:USERPROFILE\cookies.json"
$WHISPER_DIR = "E:\Models\faster-whisper-large-v3"

# DeepSeek 翻译（设置 ANTHROPIC_AUTH_TOKEN）
# 从 bitwarden 获取: 
$env:ANTHROPIC_AUTH_TOKEN=$(bwsw get -f DEE)
# Or set manually 
# $env:ANTHROPIC_AUTH_TOKEN = "your-deepseek-api-key"
# 或使用 Ark 翻译（设置 ARK_API_KEY）
$env:ARK_API_KEY = "your-ark-api-key"

# === 运行完整流水线 ===
yt-2-bili transfer `
  --cookie $COOKIE `
  --generate-subtitles `
  --subtitle-target-language zh `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  $TEST_URL

# 可选：上传后清理临时文件
yt-2-bili transfer `
  --cookie $COOKIE `
  --generate-subtitles `
  --subtitle-target-language zh `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  --cleanup `
  $TEST_URL
```

---

## 分步测试（Stage 1-6）

需要精细控制每一步时使用下面的 stage。

---

## 快捷环境变量预设

开始测试前，根据你用的 API 复制对应区块：

```powershell
# === 公共变量 ===
$env:TEST_URL = "https://www.youtube.com/watch?v=mGEfasQl2Zo"
$COOKIE = "$env:USERPROFILE\cookies.json"
$WHISPER_DIR = "E:\Models\faster-whisper-large-v3"

# === DeepSeek API ===
$env:ANTHROPIC_AUTH_TOKEN = "your-deepseek-api-key"
# 或从 bitwarden 获取: $env:ANTHROPIC_AUTH_TOKEN=$(bwsw get -f DEE)

# === 或 Ark API ===
$env:ARK_API_KEY = "your-ark-api-key"
```

---

## 环境准备

```powershell
# 1. 构建
go build -o yt-2-bili.exe ./cmd/yt-2-bili

# 2. 部署到 PATH（可选）
.\deploy.ps1

# 3. 登录 Bilibili（仅 upload/transfer 需要）
biliup login
```

## 测试用视频

```powershell
$TEST_URL = "https://www.youtube.com/watch?v=mGEfasQl2Zo"
$COOKIE = "$env:USERPROFILE\cookies.json"
$WHISPER_DIR = "E:\Models\faster-whisper-large-v3"
```

---

## Stage 1 — 下载（纯下载，无字幕）

```powershell
yt-2-bili download $TEST_URL
```

**验证**：检查输出目录生成了 `<video-id>.mp4`

```powershell
# 强制重新下载
yt-2-bili download --force-download $TEST_URL
```

---

## Stage 2 — 下载 + 生成字幕（Whisper）

```powershell
yt-2-bili download `
  --generate-subtitles `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  $TEST_URL
```

**验证**：检查生成了 `<video-id>.srt` 和 `<video-id>.burned.mp4`

```powershell
# CPU 优化（float32 更高精度）
yt-2-bili download `
  --generate-subtitles `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  --whisper-device cpu `
  --whisper-compute-type float32 `
  $TEST_URL
```

---

## Stage 3 — 本地视频生成字幕

```powershell
# 基础字幕生成
yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  video.mp4

# 强制重新生成
yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  --force-subtitles `
  video.mp4
```

**验证**：检查生成了 `<video-id>.srt` 和 `<video-id>.burned.mp4`

---

## Stage 4 — 本地视频 + 翻译中文字幕

```powershell
# DeepSeek API（设置 ANTHROPIC_AUTH_TOKEN）
$env:ANTHROPIC_AUTH_TOKEN = "your-deepseek-api-key"
# 或从 bitwarden 获取: $env:ANTHROPIC_AUTH_TOKEN=$(bwsw get -f DEE)

yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  --subtitle-target-language zh `
  video.mp4
```

```powershell
# 或使用 Ark API（设置 ARK_API_KEY）
$env:ARK_API_KEY = "your-ark-api-key"

yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  --subtitle-target-language zh `
  video.mp4
```

**验证**：检查生成了 `<video-id>.srt`、`<video-id>.zh.srt`、`<video-id>.zh.burned.mp4`

```powershell
# 可选：覆盖 LLM 模型
yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory $WHISPER_DIR `
  --subtitle-target-language zh `
  --llm-model-name "deepseek-v4-pro" `
  video.mp4
```

---

## Stage 5 — 上传到 Bilibili

```powershell
yt-2-bili upload `
  --cookie $COOKIE `
  --title "测试视频" `
  video.mp4
```

**验证**：检查 Bilibili 投稿管理中出现新视频

```powershell
# 带完整元数据上传
yt-2-bili upload `
  --cookie $COOKIE `
  --title "测试视频" `
  --desc "视频描述" `
  --cover cover.jpg `
  --source "https://www.youtube.com/watch?v=xxx" `
  --tag "标签1,标签2" `
  --tid 171 `
  video.mp4
```

---

## Stage 6 — 上传 + 生成字幕

```powershell
yt-2-bili upload `
  --cookie $COOKIE `
  --title "测试视频" `
  --generate-subtitles `
  video.mp4
```

---

## Whisper 裸调测试

跳过 yt-2-bili，直接验证 Whisper 是否正常：

```powershell
whisper-ctranslate2 `
  "E:\Projects\GoProject\yt-2-bili\sandbox\01.mp4" `
  --model_directory $WHISPER_DIR `
  --output_format srt `
  --output_dir "E:\Projects\GoProject\yt-2-bili\sandbox" `
  --device cpu `
  --compute_type int8 `
  --batched True `
  --vad_filter True
```
