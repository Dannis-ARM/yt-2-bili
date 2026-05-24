# yt-2-bili Handoff Document

Date: 2026-05-25

## Current Status

Chinese subtitle translation via Doubao (Volcengine Ark API) has been implemented and all unit tests pass. The feature is wired through the CLI and workflow, but has NOT been end-to-end tested against the real API.

## What Changed Since Last Handoff

### Chinese subtitle translation (`--subtitle-target-language zh`)

Added a streaming LLM translation path that runs after Whisper produces `<video-id>.srt`:

- **New file**: `internal/subtitle/translator.go` — Ark/OpenAI-compatible HTTP streaming client with SRT batching, validation, and retry logic.
- **Modified**: `internal/subtitle/subtitle.go` — `Options` now has `SubtitleTargetLanguage` and `Translator` (interface); `Result` has `ChineseSubtitlePath` and `ReusedChineseSubtitle`; new `prepareSubtitleFiles` handles the full source/chinese subtitle file lifecycle.
- **Modified**: `internal/workflow/workflow.go` — `Options` has `SubtitleTargetLanguage` and `Translator`; passes them through to subtitle and cleans up Chinese subtitle artifacts.
- **Modified**: `cmd/yt-2-bili/main.go` — new flags `--subtitle-target-language` and `--llm-model-name`; `validateFlags` enforces target-language requires generate-subtitles; `makeTranslator` reads `ARK_API_KEY` from env.
- **New ADR**: `docs/adr/0002-translate-subtitles-with-doubao.md` captures the full design rationale.
- **Updated**: `README.md` and `CONTEXT.md`.

### Key design decisions (see ADR for full rationale)

- `--subtitle-target-language` requires `--generate-subtitles` (validated early, before external calls)
- Only `zh` (Simplified Chinese) is supported; other values error out
- `ARK_API_KEY` env var is required only when translation is requested; missing key stops execution
- Default model: `doubao-seed-2-0-pro-260215`, base URL hardcoded to `https://ark.cn-beijing.volces.com/api/coding/v3`
- Translation in batches of 120,000 chars, preserves SRT block boundaries
- Each batch: 5 min timeout, up to 3 attempts for network/5xx/429/structure errors; 4xx not retried
- All outputs written via temp files, replaced atomically after validation
- Chinese SRT reused only when block count, numbering, and timelines match source SRT exactly
- Chinese subtitle embedding uses `mov_text` soft subtitles

### File naming convention

| Artifact | Path |
|---|---|
| Source SRT | `<video-id>.srt` |
| Chinese SRT | `<video-id>.zh.srt` |
| Source subtitled MP4 | `<video-id>.subtitled.mp4` |
| Chinese subtitled MP4 | `<video-id>.zh.subtitled.mp4` |

## Validation Already Run

```text
go test ./...
go build -o yt-2-bili.exe ./cmd/yt-2-bili
```

All pass. Tests cover:
- `TestTranslatorStreamsChineseSRT` — basic streaming translation
- `TestTranslatorRejectsChangedTimeline` — timeline mismatch rejection
- `TestTranslatorSplitsBatchesWithoutSplittingBlocks` — batch splitting with mock
- `TestTranslatorRetriesInvalidStructure` — retry on structure validation failure
- `TestTranslatorDoesNotRetryAuthenticationErrors` — 4xx stops immediately
- `TestEnsureSoftSubtitledReusesValidatedChineseSubtitle` — reuse logic
- `TestPrepareSubtitleFilesTranslatesWhenNoChineseSRT` — translator invocation
- `TestSubtitleTargetLanguageRequiresGenerateSubtitles` — CLI flag validation

## Known Issues / Watch Items

- **Not end-to-end tested**: The translation path has only been tested against fake SSE servers. Real Ark API integration has not been verified.
- Subtitled MP4 reuse logic changed for Chinese mode: old `subtitled.mp4` is NOT reused when `--subtitle-target-language zh` is set, even if it has a subtitle stream. This avoids uploading the wrong subtitle language.
- Whisper model download (from previous handoff) still needs validation.
- Bilibili soft subtitle acceptance (from previous handoff) still needs validation.

## Suggested Next Steps

1. End-to-end test with real Ark API:
   ```powershell
   $env:ARK_API_KEY = "your-key"
   .\yt-2-bili.exe transfer --generate-subtitles --subtitle-target-language zh --cookie $env:USERPROFILE\cookies.json https://www.youtube.com/watch?v=mGEfasQl2Zo
   ```
2. Run the Python integration test script (update `subtitles.py` to pass `--subtitle-target-language zh`).
3. Verify Bilibili accepts the `.zh.subtitled.mp4` with soft subtitles.
4. Test edge cases: very long video (multi-batch), network interruption mid-stream, corrupted API response.

## Suggested Skills

- `/tdd` — if adding more tests around translation edge cases or adding support for more target languages
- `/verify` — to manually test the implemented feature with real API and check Bilibili acceptance
- `/review` — before opening a pull request
- `/simplify` — after real-world testing, to tighten the streaming client and batch logic
