# Translate subtitles with Doubao after local transcription

Date: 2026-05-24

## Status

Accepted

We will keep Whisper as the local producer of Source Subtitles, then optionally translate those subtitles to Chinese with Doubao through the Volcengine Ark API when the user requests a target subtitle language. The Chinese Subtitle is written as a separate `<video-id>.zh.srt` artifact and embedded for upload, while the Source Subtitle remains available for reuse and review.

## Considered Options

- Translate the whole SRT in one request: simpler, but long videos can exceed model context limits and fail late.
- Translate SRT batches: slightly more orchestration, but keeps requests bounded and allows each batch to be validated before assembling the final subtitle.

## Consequences

Translation is explicit opt-in through `--subtitle-target-language zh`, requires `ARK_API_KEY`, uses streaming responses without printing model output, and must preserve SRT block count, numbering, and time ranges exactly. The first version only supports Simplified Chinese, writes `<video-id>.zh.srt`, embeds it into `<video-id>.zh.subtitled.mp4`, reuses the Chinese SRT only when it passes strict validation against the Source Subtitle, and translates in internal batches of up to 120,000 characters without exposing batch sizing as a CLI option. Each batch has a five-minute timeout and retries network failures, 429, 5xx responses, and structure validation failures up to two times; 4xx configuration or authentication failures are not retried. Subtitle and video outputs are written through temporary files and only replace final artifacts after validation. Translation failure stops the upload rather than falling back to publishing a video with the wrong subtitle language, and translation does not automatically change Bilibili metadata. Errors may include bounded input/output excerpts for debugging, while successful transfer cleanup removes both source and Chinese subtitle artifacts.
