# 3. Burned subtitles by default for Bilibili compatibility

Date: 2026-05-26

## Status

Accepted

## Context

Bilibili strips soft subtitle tracks from uploaded MP4 files. Users reported that videos with embedded soft subtitles appeared without subtitles after being published on Bilibili, even though the same files played correctly locally with subtitle tracks intact.

We previously only supported soft subtitles (embedded `mov_text` tracks in MP4 containers) because:
- Soft subtitles are reversible (viewers can turn them on/off)
- No video re-encoding is required (faster, no quality loss)
- Subtitle tracks can be extracted later for reuse/editing

## Decision

We will support both burned subtitles and soft subtitles:
- **Default**: Burned subtitles (text rendered directly into video frames) when using `--generate-subtitles`
- **Optional**: Soft subtitles via `--subtitle-mode=soft`
- **Alias**: `--subtitle-mode=hard` is accepted as an alias for `burned`

File naming:
- Burned subtitle video: `<video-id>.burned.mp4` / `<video-id>.zh.burned.mp4`
- Soft subtitle video: `<video-id>.subtitled.mp4` / `<video-id>.zh.subtitled.mp4` (existing behavior)

Implementation details:
- Use ffmpeg's `subtitles` filter to burn SRT files into video frames
- Font fallback strategy: prioritize system CJK fonts (Microsoft YaHei on Windows, Noto Sans CJK SC on Linux/macOS), fall back to ffmpeg defaults
- Burned subtitle rendering requires video re-encoding (slower than soft subtitle embedding)

All intermediate artifacts are still retained by default (`.srt` files, original video, etc.) for debugging and reuse.

## Consequences

### Positive
- Subtitles will definitely appear on Bilibili videos
- Users still have the option to use soft subtitles for other platforms
- Existing files with `.subtitled.mp4` naming are still compatible

### Negative
- Burned subtitle generation is slower due to video re-encoding
- Slightly larger file sizes (negligible for most use cases)
- Burned subtitles cannot be turned off by viewers
