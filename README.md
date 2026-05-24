# yt-2-bili

A Go command-line tool that uses `yt-dlp` to download YouTube videos and `biliup-rs` to upload videos to Bilibili.

## Requirements

Install these tools first and make sure they are available in `PATH`:

- `yt-dlp`
- `bun` for yt-dlp JavaScript runtime
- `biliup-rs`

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

### Upload to Bilibili

```bash
yt-2-bili upload --cookie cookies.json --title "Video title" video.mp4
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

## Common Options

| Option | Description |
| --- | --- |
| `-c, --cookie` | Path to biliup `cookies.json`; defaults to `cookies.json` in the current directory if it exists. |
| `-o, --output-dir` | Directory for downloaded files. Default on this machine: `C:\Users\18905\AppData\Local\Temp\yt-2-bili`. |
| `-q, --quality` | Video quality: `1080p`, `720p`, `480p`, or `best`. Default: `1080p`. |
| `-t, --tid` | Bilibili category ID. Default: `171`. |
| `--keep-video` | Keep downloaded files after a successful `transfer`. |
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
