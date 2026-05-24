# yt-2-bili Handoff Document

Date: 2026-05-24

## Current Status

The project is now a Go CLI that coordinates external tools for YouTube download, Bilibili upload, and optional local subtitle generation. The main implementation has been built and validated with unit tests and local builds, but the newest subtitle flow has not yet been manually end-to-end tested by this agent.

## Key Decisions

Reference existing artifacts instead of duplicating full rationale:

- Domain glossary: `CONTEXT.md`
- External tool architecture: `docs/adr/0001-use-external-tools.md`
- CLI usage and integration test commands: `README.md`

Important current decisions:

- Use external CLIs instead of reimplementing platform protocols:
  - `yt-dlp` for YouTube download
  - `biliup` for Bilibili upload
  - `whisper` CLI from `openai-whisper` for subtitle generation
  - `ffmpeg` / `ffprobe` for soft subtitle embedding and validation
- Default download output is MP4/M4A merged to MP4 for Bilibili compatibility.
- `--generate-subtitles` is supported on `download`, `upload`, and `transfer`.
- Subtitle flow:
  - generate `<base>.srt` with `whisper`
  - embed it as a soft subtitle track into `<base>.subtitled.mp4`
  - use `ffprobe` to reuse an existing valid subtitled MP4
  - reuse existing SRT and only re-embed if needed
- No `.bcc` support in the current version.
- Default behavior is to keep generated/downloaded files.
- `--keep-video` was replaced by `--cleanup`; cleanup only runs after successful `transfer`, and upload failures keep artifacts.

## Current Project Structure Highlights

- `cmd/yt-2-bili/main.go`
  - Cobra CLI setup
  - Commands: `download`, `upload`, `transfer`
  - Common flags include `--generate-subtitles`, Whisper options, `--cleanup`, `--force-download`
- `internal/ytdlp/ytdlp.go`
  - Calls `yt-dlp`
  - Uses `--js-runtime bun`
  - Prefers MP4/M4A and `--merge-output-format mp4`
  - Reuses existing non-empty `<video-id>.mp4` unless `--force-download` is set
- `internal/biliup/biliup.go`
  - Calls `biliup`
  - Normalizes upload metadata for Bilibili-friendly lengths and tag limits
- `internal/workflow/workflow.go`
  - Coordinates transfer flow
  - Converts WebP covers to JPG before upload
  - Uses subtitled MP4 for upload when `--generate-subtitles` is enabled
- `internal/subtitle/subtitle.go`
  - Calls `whisper`, `ffmpeg`, `ffprobe`
  - Ensures final video has a soft subtitle track
- `integration-tests/`
  - Python integration scripts for real workflows
  - `subtitles.py` tests subtitle generation path
- `install-whisper.ps1`
  - Installs `openai-whisper` via `uv`
  - Downloads the model from a Hugging Face mirror through a temporary file before moving to `~/.cache/whisper/large-v3.pt`
  - Adds the mirror host to `NO_PROXY` / `no_proxy`

## Validation Already Run

The following checks passed after subtitle implementation:

```text
go test ./...
python -m py_compile integration-tests/helpers.py integration-tests/download.py integration-tests/upload.py integration-tests/e2e.py integration-tests/subtitles.py
go build -o yt-2-bili.exe ./cmd/yt-2-bili
```

A built `yt-2-bili.exe` may exist in the repository root from the latest build.

## Known Issues / Watch Items

- The Whisper model download in `install-whisper.ps1` targets `https://hf-mirror.com/openai/whisper-large-v3-turbo/resolve/main/pytorch_model.bin?download=true` but saves as `~/.cache/whisper/large-v3.pt`. Verify that `openai-whisper` accepts this file format/name before relying on it.
- The script still contains a TODO about trying `whisper.cpp`; current implementation uses `openai-whisper` CLI named `whisper`.
- Bilibili upload previously failed with `code: -400 请求错误`; metadata normalization and WebP-to-JPG cover conversion were added afterward, but the user should retest real upload.
- The subtitle embedding uses MP4 `mov_text` soft subtitles. Confirm Bilibili preserves or accepts this as expected.
- The current `download.py`, `upload.py`, and `e2e.py` integration scripts do not enable subtitles by default; `subtitles.py` is the dedicated subtitle integration test.

## Suggested Next Steps

1. Run the dedicated subtitle integration test:
   ```powershell
   python integration-tests\subtitles.py
   ```
2. Retest real transfer with subtitles:
   ```powershell
   .\yt-2-bili.exe transfer --generate-subtitles --cookie $env:USERPROFILE\cookies.json https://www.youtube.com/watch?v=mGEfasQl2Zo
   ```
3. Validate `install-whisper.ps1` model download actually produces a model usable by `openai-whisper`.
4. If Bilibili rejects soft-subtitled MP4, consider falling back to uploading original MP4 plus keeping SRT, or investigating biliup subtitle-file support.
5. Consider adding a collision/archive feature later to avoid reposting videos already uploaded by the user.

## Suggested Skills

- `/tdd` — for adding tests around subtitle edge cases, cleanup behavior, or archive/collision detection.
- `/grill-with-docs` — for clarifying future domain decisions like repost collision detection or Bilibili subtitle handling.
- `/simplify` — after more real-world testing, to review and clean up the external-tool orchestration code.
- `/review` — before opening a pull request.
