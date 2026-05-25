# CAVEATS

## Incremental subtitle translation workflow is supported

You can generate subtitles first, then decide to translate them later without repeating work.

### Example scenario

**First run (no translation):**
```bash
yt-2-bili transfer --generate-subtitles <youtube-url>
```
- Generates `<id>.srt` (source subtitle)
- Generates `<id>.subtitled.mp4` (with source subtitle embedded)

**Second run (with translation):**
```bash
yt-2-bili transfer --generate-subtitles --subtitle-target-language zh <youtube-url>
```
- Reuses existing `<id>.srt` (no Whisper re-run)
- Generates `<id>.zh.srt` (translated subtitle)
- Generates `<id>.zh.subtitled.mp4` (with Chinese subtitle embedded)

### Why this works

1. **Separate output paths**: English subtitled video goes to `<id>.subtitled.mp4`, Chinese subtitled video goes to `<id>.zh.subtitled.mp4`. They don't overwrite each other.

2. **Smart reuse check in `EnsureSoftSubtitled()`**:
   ```go
   if hasSubtitleStream(result.SubtitledVideoPath) && opts.SubtitleTargetLanguage == "" {
       // Only reuse when NOT translating
   }
   ```
   The check only skips embedding when:
   - The expected output file already has a subtitle stream, AND
   - We're NOT requesting translation

3. **Subtitle file reuse**:
   - Source subtitle `<id>.srt` is always reused if present
   - Chinese subtitle `<id>.zh.srt` is reused if present AND passes validation against source

### Key files

- `<id>.srt` - Source subtitle (kept as intermediate artifact)
- `<id>.zh.srt` - Chinese subtitle (only when translation requested)
- `<id>.subtitled.mp4` - Video with source subtitle
- `<id>.zh.subtitled.mp4` - Video with Chinese subtitle
