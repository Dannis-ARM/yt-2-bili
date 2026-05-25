# yt-2-bili Handoff Document

Date: 2026-05-26

## Current Status

Whisper performance optimization has been implemented and all tests pass. The changes add default optimization parameters and two new CLI flags for device and compute type configuration.

## What Changed Since Last Handoff

### Whisper Performance Optimization

Added default optimization parameters to `whisper-ctranslate2` invocation and new CLI flags for customizing behavior:

- **Modified**: `internal/subtitle/subtitle.go` - Added `WhisperDevice` and `WhisperComputeType` to `Options`; updated `buildWhisperArgs` to include:
  - `--compute_type int8` (default, can be overridden)
  - `--batched True`
  - `--vad_filter True`
- **Modified**: `cmd/yt-2-bili/main.go` - Added two new persistent flags:
  - `--whisper-device`: Passed as `--device` to whisper CLI (e.g. `cpu`, `cuda`)
  - `--whisper-compute-type`: Overrides default `int8` (e.g. `float32`, `float16`)
- **Modified**: `internal/workflow/workflow.go` - Added fields to `Options` and pass-through to subtitle generation
- **Updated**: `README.md` - Added documentation for new flags and optimization notes
- **Updated**: `internal/subtitle/subtitle_test.go` - Added tests for new parameter ordering and device/compute-type flags

### Usage Examples

Default optimized mode (int8, batched, vad_filter):
```powershell
yt-2-bili download --generate-subtitles --whisper-model-directory "E:\Models\faster-whisper-large-v3" <youtube-url>
```

CPU with float32 for better accuracy (6800X3D/7800X3D):
```powershell
yt-2-bili download --generate-subtitles --whisper-device cpu --whisper-compute-type float32 --whisper-model-directory "E:\Models\faster-whisper-large-v3" <youtube-url>
```

## Validation Already Run

```text
go test ./...
```

All pass. Tests cover the new parameter behavior.

## Known Issues / Watch Items

- **No DirectML support**: User's `whisper-ctranslate2` only supports `auto`, `cpu`, `cuda`; no DirectML option for AMD GPU (7900GRE). Currently stuck on CPU.
- **Performance baseline**: User reported 10 minutes to process 10 minutes of video with large-v3 on CPU before optimization.

## Suggested Next Steps

1. Test the new optimized default settings and see if performance improves from the 1x realtime baseline.
2. Explore alternative Whisper implementations that support AMD GPU on Windows (e.g. DirectML-optimized versions).
3. Consider adding model size option (medium/small) for faster transcription when quality trade-off is acceptable.

## Suggested Skills

- `/verify` - Test the new optimization parameters with a real video file
- `/simplify` - After real-world testing, review if any further parameter tuning is needed
