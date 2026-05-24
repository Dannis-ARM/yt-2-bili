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

This creates:

```text
<video-id>.mp4
<video-id>.srt
<video-id>.subtitled.mp4
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

With `--generate-subtitles`, it generates `<video-id>.srt`, embeds it into `<video-id>.subtitled.mp4`, and uploads the subtitled MP4.

Use `whisper-ctranslate2` with a local faster-whisper model directory during transfer:

```powershell
yt-2-bili transfer `
  --cookie cookies.json `
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
| `--generate-subtitles` | Generate an SRT file and embed it as a soft subtitle into an MP4. |
| `--whisper-path` | Path to the Whisper-compatible executable if it is not in `PATH`. |
| `--whisper-model-directory` | Local model directory for Whisper-compatible CLIs that support `--model_directory`. |
| `--yt-dlp-path` | Path to the `yt-dlp` executable if it is not in `PATH`. |
| `--biliup-path` | Path to the `biliup` executable if it is not in `PATH`. |

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
