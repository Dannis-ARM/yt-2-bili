# yt-2-bili

A Go command-line tool that uses `yt-dlp` to download YouTube videos and `biliup-rs` to upload videos to Bilibili.

## Requirements

Install these tools first and make sure they are available in `PATH`:

- `yt-dlp`
- `bun` for yt-dlp JavaScript runtime
- `biliup-rs`

Optional for subtitle generation:

- `whisper` or `whisper-ctranslate2`
- `ffmpeg`
- `ffprobe`

Install faster Whisper and download the default local model:

```powershell
.\install-faster-whisper.ps1
```

The script downloads `Systran/faster-whisper-large-v3` to `E:\Models\faster-whisper-large-v3` via `https://hf-mirror.com` by default. Choose a different model directory with `-LocalDir`, then pass the same path to `--whisper-model-directory` when using `whisper-ctranslate2`.

```powershell
.\install-faster-whisper.ps1 -LocalDir "D:\Models\faster-whisper-large-v3"
```

Log in to Bilibili before uploading:

```bash
biliup login
```

This creates a `cookies.json` file that `yt-2-bili` can use.

## Build

```bash
go build -o yt-2-bili.exe ./cmd/yt-2-bili
```

## Commands

### Download from YouTube

```bash
yt-2-bili download <youtube-url>
```

Downloads the video, metadata, and thumbnail using `yt-dlp`. By default, it prefers MP4/M4A formats and merges output to MP4 for better Bilibili compatibility.

If `<video-id>.mp4` already exists in the output directory and is not empty, the existing file is reused. Use `--force-download` to download again. `yt-dlp`'s default partial download resume behavior remains enabled.

Generate subtitles during download:

```bash
yt-2-bili download --generate-subtitles <youtube-url>
```

Use `whisper-ctranslate2` with a local faster-whisper model directory:

```powershell
yt-2-bili download `
  --generate-subtitles `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  <youtube-url>
```

Optimize for CPU (6800X3D/7800X3D) with `float32` for better accuracy:

```powershell
yt-2-bili download `
  --generate-subtitles `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  --whisper-device cpu `
  --whisper-compute-type float32 `
  <youtube-url>
```

This creates:

```text
<video-id>.mp4
<video-id>.srt
<video-id>.subtitled.mp4
```

### Generate subtitles for local video

```bash
yt-2-bili subtitle <video-file>
```

Generate subtitles for a local video file without downloading or uploading. This is useful for batch processing videos locally before uploading.

Use `whisper-ctranslate2` with a local faster-whisper model directory:

```powershell
yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  video.mp4
```

Translate subtitles to Simplified Chinese:

```powershell
$env:ARK_API_KEY = "your-api-key"

yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  --subtitle-target-language zh `
  video.mp4
```

Force regenerate subtitles even if files already exist:

```powershell
yt-2-bili subtitle `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  --subtitle-target-language zh `
  --force-subtitles `
  video.mp4
```

This creates:
- `<video-id>.srt` - Source subtitles
- `<video-id>.zh.srt` - Chinese subtitles (if translated)
- `<video-id>.subtitled.mp4` - Video with soft subtitles
- `<video-id>.zh.subtitled.mp4` - Video with Chinese soft subtitles (if translated)

#### Translate subtitles to Simplified Chinese

When `--subtitle-target-language zh` is set with `--generate-subtitles`, the source SRT is sent to Doubao via the Volcengine Ark API for translation. Each batch preserves block count, numbering, and time ranges exactly; translation failure stops the upload.

```powershell
# Set your Ark API key
$env:ARK_API_KEY = "your-api-key"

yt-2-bili transfer `
  --cookie $env:USERPROFILE\cookies.json `
  --generate-subtitles `
  --subtitle-target-language zh `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  <youtube-url>
```

Optional: override the LLM model (default is `doubao-seed-2-0-pro-260215`).

```powershell
yt-2-bili transfer `
  --generate-subtitles `
  --subtitle-target-language zh `
  --llm-model-name "doubao-seed-2-0-pro-260215" `
  <youtube-url>
```

This creates:

```text
<video-id>.mp4
<video-id>.srt              # Source SRT, kept as intermediate artifact
<video-id>.zh.srt            # Chinese SRT
<video-id>.zh.subtitled.mp4  # Video with Chinese soft subtitle
```

### Upload to Bilibili

```bash
yt-2-bili upload --cookie cookies.json --title "Video title" video.mp4
```

Generate and upload a soft-subtitled MP4:

```bash
yt-2-bili upload --generate-subtitles --cookie cookies.json --title "Video title" video.mp4
```

Useful options:

```bash
yt-2-bili upload \
  --cookie cookies.json \
  --title "Video title" \
  --desc "Video description" \
  --cover cover.jpg \
  --source "https://www.youtube.com/watch?v=..." \
  --tag "tag1,tag2" \
  --tid 171 \
  video.mp4
```

### Transfer from YouTube to Bilibili

```bash
yt-2-bili transfer --cookie cookies.json <youtube-url>
```

This downloads the YouTube video, builds a Bilibili description containing the original author and YouTube link, then uploads it to Bilibili.

With `--generate-subtitles`, it generates `<video-id>.srt`, embeds it into `<video-id>.subtitled.mp4`, and uploads the subtitled MP4. Add `--subtitle-target-language zh` to translate the source subtitles to Simplified Chinese before embedding.

Use `whisper-ctranslate2` with a local faster-whisper model directory during transfer:

```powershell
yt-2-bili transfer `
  --cookie $env:USERPROFILE\cookies.json `
  --generate-subtitles `
  --whisper-path whisper-ctranslate2 `
  --whisper-model-directory "E:\Models\faster-whisper-large-v3" `
  <youtube-url>
```

## Common Options

| Option | Description |
| --- | --- |
| `-c, --cookie` | Path to biliup `cookies.json`; defaults to `cookies.json` in the current directory if it exists. |
| `-o, --output-dir` | Directory for downloaded files. Default on this machine: `C:\Users\18905\AppData\Local\Temp\yt-2-bili`. |
| `-q, --quality` | Video quality: `1080p`, `720p`, `480p`, or `best`. Default: `1080p`. |
| `-t, --tid` | Bilibili category ID. Default: `171`. |
| `--cleanup` | Clean up generated files after a successful `transfer`; by default files are kept. |
| `--force-download` | Download again even if the expected local video file already exists. |
| `--force-subtitles` | Force regenerate subtitles even if files already exist. |
| `--generate-subtitles` | Generate an SRT file and embed it as a soft subtitle into an MP4. |
| `--whisper-path` | Path to the Whisper-compatible executable if it is not in `PATH`. |
| `--whisper-model-directory` | Local model directory for Whisper-compatible CLIs that support `--model_directory`. |
| `--whisper-device` | Device for Whisper (e.g. `cpu`, `cuda`). Passed as `--device` if set. |
| `--whisper-compute-type` | Compute type for Whisper (e.g. `int8`, `float16`, `float32`). Overrides default `int8`. |
| `--subtitle-target-language` | Target language for subtitle translation (e.g. `zh`). Requires `--generate-subtitles`. |
| `--llm-model-name` | LLM model for subtitle translation. Default: `doubao-seed-2-0-pro-260215`. |
| `--yt-dlp-path` | Path to the `yt-dlp` executable if it is not in `PATH`. |
| `--biliup-path` | Path to the `biliup` executable if it is not in `PATH`. |

**Whisper Optimization**: By default, `--compute_type int8`, `--batched True`, and `--vad_filter True` are used for faster transcription.

## Integration Tests

The integration test scripts live under `integration-tests/` and use Python.

These scripts run the real CLI. `upload.py` and `e2e.py` perform real Bilibili uploads with the cookie file.

Default test video:

```text
https://www.youtube.com/watch?v=mGEfasQl2Zo
```

Default cookie path:

```text
%USERPROFILE%\cookies.json
```

Run download only:

```powershell
python integration-tests\download.py
```

Run subtitle generation test:

```powershell
python integration-tests\subtitles.py
```

Run real upload using a generated short test video:

```powershell
python integration-tests\upload.py
```

Run real end-to-end download and upload:

```powershell
python integration-tests\e2e.py
```

Override cookie path:

```powershell
python integration-tests\upload.py C:\path\to\cookies.json
python integration-tests\e2e.py C:\path\to\cookies.json
```

Override end-to-end URL:

```powershell
python integration-tests\e2e.py C:\path\to\cookies.json https://www.youtube.com/watch?v=VIDEO_ID
```

## Notes

- Playlist URLs are not supported yet.
- Uploads use copyright mode `2`, meaning reupload/repost.
- The Bilibili source field is set to the original YouTube URL during `transfer`.

## Whisper Test

Test direct `whisper-ctranslate2` invocation (same options as yt-2-bili uses by default):

```powershell
whisper-ctranslate2 `
  "E:\Projects\GoProject\yt-2-bili\sandbox\01.mp4" `
  --model_directory "E:\Models\faster-whisper-large-v3" `
  --output_format srt `
  --output_dir "E:\Projects\GoProject\yt-2-bili\sandbox" `
  --device cpu `
  --compute_type int8 `
  --batched True `
  --vad_filter True
```

For better accuracy with 6800X3D/7800X3D (slower), use `float32`:

```powershell
whisper-ctranslate2 `
  "E:\Projects\GoProject\yt-2-bili\sandbox\01.mp4" `
  --model_directory "E:\Models\faster-whisper-large-v3" `
  --output_format srt `
  --output_dir "E:\Projects\GoProject\yt-2-bili\sandbox" `
  --device cpu `
  --compute_type float32 `
  --batched True `
  --vad_filter True
```
