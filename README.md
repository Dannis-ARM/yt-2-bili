# yt-2-bili

YouTube → Bilibili 搬运工具，基于 `yt-dlp` + `biliup-rs`。

## Requirements

- `yt-dlp` + `bun`（JavaScript runtime）
- `biliup-rs`（上传用）
- 可选：`whisper-ctranslate2`、`ffmpeg`、`ffprobe`（字幕用）

```powershell
.\install-faster-whisper.ps1   # 安装 faster-whisper
biliup login                    # 登录 Bilibili
```

## Build & Deploy

```bash
go build -o yt-2-bili.exe ./cmd/yt-2-bili
```

```powershell
.\deploy.ps1   # 部署到 $env:USERPROFILE\.local\bin
```

## Commands

| Command | Description |
|---------|-------------|
| `download <url>` | 下载 YouTube 视频 |
| `subtitle <file>` | 本地视频生成字幕 |
| `upload --cookie <path> --title <title> <file>` | 上传到 Bilibili |
| `transfer --cookie <path> <url>` | 完整流水线（下载→字幕→翻译→上传） |

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `-c, --cookie` | `./cookies.json` | biliup cookies 路径 |
| `-o, --output-dir` | `%TEMP%\yt-2-bili` | 输出目录 |
| `-q, --quality` | `1080p` | 画质：`1080p` / `720p` / `480p` / `best` |
| `-t, --tid` | `171` | Bilibili 分区 ID |
| `--generate-subtitles` | — | 生成字幕并嵌入视频 |
| `--subtitle-mode` | `burned` | `burned`（硬字幕）/ `soft`（软字幕） |
| `--subtitle-target-language` | — | 翻译目标语言（如 `zh`），需 `ANTHROPIC_AUTH_TOKEN` 或 `ARK_API_KEY` |
| `--whisper-path` | — | Whisper 可执行文件路径 |
| `--whisper-model-directory` | — | 本地模型目录 |
| `--whisper-device` | — | `cpu` / `cuda` |
| `--whisper-compute-type` | `int8` | `int8` / `float16` / `float32` |
| `--llm-model-name` | `deepseek-v4-pro` | 翻译用 LLM 模型 |
| `--force-download` | — | 强制重新下载 |
| `--force-subtitles` | — | 强制重新生成字幕 |
| `--cleanup` | — | transfer 成功后清理临时文件 |
| `--yt-dlp-path` | — | yt-dlp 路径 |
| `--biliup-path` | — | biliup 路径 |

## 测试

[TESTING.md](TESTING.md)

## Notes

- 不支持播放列表 URL
- 上传使用版权模式 `2`（转载）
