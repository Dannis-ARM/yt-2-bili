# Handoff: yt-2-bili Architecture Improvement

**Date**: 2026-05-29
**Branch**: main
**Last commit**: `33489cb bug fix audio 2`

## Summary

Executed the `/improve-codebase-architecture` plan. The architecture review identified 4 candidates; we completed the two "Strong" recommendations.

## Completed

### Candidate 1: ffmpeg module (DONE)
- Created `internal/ffmpeg/ffmpeg.go` — consolidates all ffmpeg operations
- Exports: `ConvertCover`, `EmbedSoftSubtitle`, `BurnSubtitle`, `HasSubtitleStream`, `CheckAvailable`
- `internal/workflow/workflow.go` — delegates cover conversion to `ffmpeg.ConvertCover`
- `internal/subtitle/subtitle.go` — delegates subtitle embedding to ffmpeg module

### Candidate 4: subtitle module split (DONE)
- Created `internal/subtitle/srt/srt.go` — SRT parse/format (`Block`, `Parse`, `ParseFile`, `Format`, `Write`, `CountBlocks`)
- Created `internal/subtitle/whisper/whisper.go` — whisper CLI integration (`Options`, `GenerateSRT`, `CheckAvailable`)
- Created `internal/subtitle/whisper/whisper_test.go` — tests for `buildArgs`
- Created `internal/subtitle/util.go` — shared helpers (`isNonEmptyFile`, `fileSizeStr`)
- `internal/subtitle/subtitle.go` — reduced from ~600 lines to ~270 lines, delegates to srt/whisper packages
- `internal/subtitle/translator.go` — updated to use `srt.Block` / `srt.Parse` / `srt.Format`
- `internal/subtitle/sentence-breaker.go` — updated to use `srt.Block` / `srt.Parse` / `srt.Format`

### Test fixes
- `sentence-breaker_test.go` — `parseSRTBlocks` → `srt.Parse`, `Timeline` string → `Start`/`End` time.Duration
- `subtitle_test.go` — removed `TestBuildWhisperArgs*` (moved to whisper package)
- All 21 tests pass, `go build ./...` succeeds

## Not Yet Done

### Deviates from architecture review
- **`break` submodule was NOT created** — `BreakSentences` remains in `internal/subtitle/sentence-breaker.go` as part of the subtitle package. The review diagram showed it as `internal/subtitle/break/`.
- **Candidate 2 (translator split)** not attempted — marked "Worth exploring". `translator.go` still combines SRT batching + LLM HTTP (OpenAI + Anthropic) in one file.
- **Candidate 3 (main.go extraction)** not attempted — marked "Speculative".

### Uncommitted changes
All changes are **uncommitted**. The diff is staged but no commits were created for the module splits.

```
M internal/subtitle/sentence-breaker.go
M internal/subtitle/subtitle.go
M internal/subtitle/translator.go
M internal/workflow/workflow.go
?? internal/ffmpeg/
?? internal/subtitle/srt/
?? internal/subtitle/util.go
?? internal/subtitle/whisper/
```

## Suggested Next Steps

1. **Create `internal/subtitle/breaker/` submodule** — extract `BreakSentences` and helpers (`splitBlockIfNeeded`, `splitBlockEvenly`, `charCount`, `extractTextLines`) into its own package. Then `sentence-breaker_test.go` would also move into that package.
2. **Commit** the module splits as separate commits for clarity.
3. Consider Candidate 2 (translator split) if translator complexity grows.

## Suggested Skills

- `/improve-codebase-architecture` — if you want to continue with the remaining candidates (translator split, main.go extraction)
- `/verify` — to verify the build and tests still pass after any further changes
- `/simplify` — to review the diff before committing

## Architecture Review Reference

Full analysis at: `C:/Users/18905/AppData/Local/Temp/architecture-review-yt-2-bili.html`
