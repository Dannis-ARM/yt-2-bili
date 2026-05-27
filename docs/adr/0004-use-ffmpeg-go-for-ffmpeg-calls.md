# 4. Use ffmpeg-go library for ffmpeg calls

Date: 2026-05-27

## Status

Accepted

## Context

We currently call ffmpeg directly via `exec.Command` with manually constructed string arguments. This approach has caused issues:

1. **Path escaping bugs** - ffmpeg's `subtitles` filter requires special escaping for colons, backslashes, and single quotes. We had an `escapeFFmpegPath` function but it wasn't being used consistently, leading to a debugging session that took an afternoon.

2. **Future complexity** - We anticipate adding more ffmpeg operations:
   - Format conversion
   - Resolution adjustment
   - Audio processing
   - Potentially complex filter graphs

3. **Precedent consistency** - ADR 0001 established using external CLI tools (yt-dlp, biliup) instead of reimplementing from scratch. Using ffmpeg-go does NOT violate this - it still uses the external ffmpeg binary, just provides a safer way to construct arguments.

## Decision

We will use `github.com/u2takey/ffmpeg-go` for constructing and executing ffmpeg commands.

### Exceptions

- **ffprobe calls** will still use `exec.Command` directly because ffmpeg-go's ffprobe support is limited and our current usage is simple and stable.

### Implementation details

1. Add ffmpeg-go as a direct dependency (it's already present as an indirect dependency)
2. Migrate these 3 ffmpeg call sites:
   - `embedSoftSubtitle` - soft subtitle embedding
   - `burnSubtitle` - hard subtitle burning
   - `prepareCoverForUpload` - webp to jpg cover conversion
3. Keep ffprobe usage (`hasSubtitleStream`) as-is
4. Encapsulate ffmpeg-go usage within the `subtitle` package (and a small part in `workflow` for cover conversion) to avoid spreading the dependency throughout the codebase

## Consequences

### Positive

- Type-safe argument construction - no more manual string concatenation
- Built-in path escaping for filter arguments
- Better support for complex filter graphs in the future
- More readable code with method chaining instead of string slices
- No breaking changes to external API

### Negative

- One more direct dependency to manage
- Slightly higher cognitive load for contributors unfamiliar with ffmpeg-go
- Debugging requires printing the compiled command (ffmpeg-go provides `Compile()` for this)

## Migration strategy

1. Write the ADR (this document)
2. Add ffmpeg-go as a direct dependency
3. Migrate one function at a time, running tests between each
4. Keep the same function signatures, only change internal implementation
5. Verify with real-world test cases (paths with spaces, special characters, non-ASCII characters)
