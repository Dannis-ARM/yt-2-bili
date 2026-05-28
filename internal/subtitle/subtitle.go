package subtitle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dannis/yt-2-bili/internal/ffmpeg"
	"github.com/dannis/yt-2-bili/internal/subtitle/srt"
	"github.com/dannis/yt-2-bili/internal/subtitle/whisper"
)

// Mode represents the subtitle embedding mode.
type Mode string

const (
	// ModeBurned renders subtitles directly into video frames.
	// Cannot be turned off by viewers, but compatible with all platforms.
	ModeBurned Mode = "burned"
	// ModeHard is an alias for ModeBurned.
	ModeHard Mode = "hard"
	// ModeSoft embeds subtitles as a separate track in the video container.
	// Can be turned on/off by viewers, but may be stripped by some platforms.
	ModeSoft Mode = "soft"
)

// ParseMode parses a string into a subtitle Mode.
// Accepts "burned", "hard", or "soft". Defaults to ModeBurned if empty.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(s) {
	case "", string(ModeBurned), string(ModeHard):
		return ModeBurned, nil
	case string(ModeSoft):
		return ModeSoft, nil
	default:
		return "", fmt.Errorf("invalid subtitle mode: %q (must be burned/hard or soft)", s)
	}
}

type Options struct {
	VideoPath              string
	WhisperPath            string
	ModelDirectory         string
	WhisperDevice          string
	WhisperComputeType     string
	SubtitleTargetLanguage string
	Translator             Translator
	Force                  bool
	SubtitleMode           Mode // Burned by default
}

type Result struct {
	OriginalVideoPath       string
	SubtitlePath            string
	ChineseSubtitlePath     string
	SubtitledVideoPath      string
	ReusedSubtitled         bool
	ReusedSubtitle          bool
	ReusedChineseSubtitle   bool
	GeneratedSourceSubtitle bool
	SubtitleMode            Mode
}

func CheckAvailable(whisperPath string) error {
	if err := whisper.CheckAvailable(whisperPath); err != nil {
		return err
	}
	if err := ffmpeg.CheckAvailable(); err != nil {
		return err
	}
	return nil
}

// EnsureSubtitled ensures the video has subtitles in the specified mode.
// If SubtitleMode is empty, it defaults to ModeBurned.
func EnsureSubtitled(opts Options) (*Result, error) {
	if opts.VideoPath == "" {
		return nil, fmt.Errorf("video path is required")
	}
	if opts.SubtitleMode == "" {
		opts.SubtitleMode = ModeBurned
	}
	if err := CheckAvailable(opts.WhisperPath); err != nil {
		return nil, err
	}

	result, err := prepareSubtitleFiles(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	result.SubtitleMode = opts.SubtitleMode

	// Determine which subtitle file to use
	subtitleForEmbedding := result.SubtitlePath
	if result.ChineseSubtitlePath != "" {
		subtitleForEmbedding = result.ChineseSubtitlePath
	}

	// Check if we can reuse existing output
	if !opts.Force {
		if opts.SubtitleMode == ModeSoft && ffmpeg.HasSubtitleStream(result.SubtitledVideoPath) {
			result.ReusedSubtitled = true
			return result, nil
		}
		if opts.SubtitleMode == ModeBurned && isNonEmptyFile(result.SubtitledVideoPath) {
			// For burned subtitles, we just check if the file exists
			// (no reliable way to verify burned-in subtitles without visual inspection)
			result.ReusedSubtitled = true
			return result, nil
		}
	}

	// Generate the subtitled video
	start := time.Now()
	if opts.SubtitleMode == ModeSoft {
		fmt.Fprintf(os.Stderr, "Embedding soft subtitles with ffmpeg... ")
		if err := ffmpeg.EmbedSoftSubtitle(opts.VideoPath, subtitleForEmbedding, result.SubtitledVideoPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		if !ffmpeg.HasSubtitleStream(result.SubtitledVideoPath) {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, fmt.Errorf("ffmpeg finished but output has no subtitle stream: %s", result.SubtitledVideoPath)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Burning subtitles with ffmpeg... ")
		if err := ffmpeg.BurnSubtitle(opts.VideoPath, subtitleForEmbedding, result.SubtitledVideoPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		if !isNonEmptyFile(result.SubtitledVideoPath) {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, fmt.Errorf("ffmpeg finished but output file was not created: %s", result.SubtitledVideoPath)
		}
	}
	fmt.Fprintf(os.Stderr, "done (%v) — %s\n", time.Since(start).Round(time.Millisecond), fileSizeStr(result.SubtitledVideoPath))

	return result, nil
}

// EnsureSoftSubtitled is kept for backward compatibility.
// It calls EnsureSubtitled with ModeSoft.
func EnsureSoftSubtitled(opts Options) (*Result, error) {
	opts.SubtitleMode = ModeSoft
	return EnsureSubtitled(opts)
}

func prepareSubtitleFiles(ctx context.Context, opts Options) (*Result, error) {
	srtPath := subtitlePath(opts.VideoPath)
	result := &Result{
		OriginalVideoPath:  opts.VideoPath,
		SubtitlePath:       srtPath,
		SubtitleMode:       opts.SubtitleMode,
		SubtitledVideoPath: subtitledVideoPathForMode(opts.VideoPath, opts.SubtitleMode),
	}

	// Delete existing files if force is enabled
	if opts.Force {
		_ = os.Remove(srtPath)
		_ = os.Remove(chineseSubtitlePath(opts.VideoPath))
		_ = os.Remove(subtitledVideoPath(opts.VideoPath))
		_ = os.Remove(chineseSubtitledVideoPath(opts.VideoPath))
		_ = os.Remove(subtitledVideoPathForMode(opts.VideoPath, ModeBurned))
		_ = os.Remove(chineseSubtitledVideoPathForMode(opts.VideoPath, ModeBurned))
	}

	reusedSubtitle := isNonEmptyFile(srtPath)
	if !reusedSubtitle {
		// Stage: Whisper SRT generation
		start := time.Now()
		fmt.Fprintf(os.Stderr, "Generating SRT with Whisper... ")
		whisperOpts := whisper.Options{
			VideoPath:          opts.VideoPath,
			WhisperPath:        opts.WhisperPath,
			ModelDirectory:     opts.ModelDirectory,
			WhisperDevice:      opts.WhisperDevice,
			WhisperComputeType: opts.WhisperComputeType,
		}
		if err := whisper.GenerateSRT(whisperOpts, srtPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		if !isNonEmptyFile(srtPath) {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, fmt.Errorf("whisper finished but subtitle file was not created: %s", srtPath)
		}
		entries, _ := srt.CountBlocks(srtPath)
		fmt.Fprintf(os.Stderr, "done (%v) — %d entries, %s\n", time.Since(start).Round(time.Millisecond), entries, fileSizeStr(srtPath))

		// Stage: Sentence breaking
		start = time.Now()
		entriesBefore, _ := srt.CountBlocks(srtPath)
		fmt.Fprintf(os.Stderr, "Applying sentence breaking... ")
		if err := applySentenceBreaking(srtPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		entriesAfter, _ := srt.CountBlocks(srtPath)
		fmt.Fprintf(os.Stderr, "done (%v) — %d → %d entries\n", time.Since(start).Round(time.Millisecond), entriesBefore, entriesAfter)
	} else {
		fmt.Fprintf(os.Stderr, "Reusing existing SRT: %s (%s)\n", srtPath, fileSizeStr(srtPath))
	}
	result.ReusedSubtitle = reusedSubtitle
	result.GeneratedSourceSubtitle = !reusedSubtitle

	if opts.SubtitleTargetLanguage == "" {
		// Auto-detect existing Chinese subtitle from a previous run
		zhPath := chineseSubtitlePath(opts.VideoPath)
		if isNonEmptyFile(zhPath) {
			sourceData, err := os.ReadFile(srtPath)
			if err != nil {
				return nil, err
			}
			translatedData, err := os.ReadFile(zhPath)
			if err != nil {
				return nil, err
			}
			if err := validateTranslatedSRT(string(sourceData), string(translatedData)); err == nil {
				result.ChineseSubtitlePath = zhPath
				result.SubtitledVideoPath = chineseSubtitledVideoPathForMode(opts.VideoPath, opts.SubtitleMode)
				result.ReusedChineseSubtitle = true
				fmt.Fprintf(os.Stderr, "Reusing existing Chinese SRT: %s (%s)\n", zhPath, fileSizeStr(zhPath))
			}
		}
		return result, nil
	}
	if opts.SubtitleTargetLanguage != "zh" {
		return nil, fmt.Errorf("unsupported subtitle target language: %s", opts.SubtitleTargetLanguage)
	}

	zhPath := chineseSubtitlePath(opts.VideoPath)
	result.ChineseSubtitlePath = zhPath
	result.SubtitledVideoPath = chineseSubtitledVideoPathForMode(opts.VideoPath, opts.SubtitleMode)
	if isNonEmptyFile(zhPath) {
		sourceData, err := os.ReadFile(srtPath)
		if err != nil {
			return nil, err
		}
		translatedData, err := os.ReadFile(zhPath)
		if err != nil {
			return nil, err
		}
		if err := validateTranslatedSRT(string(sourceData), string(translatedData)); err == nil {
			result.ReusedChineseSubtitle = true
			fmt.Fprintf(os.Stderr, "Reusing existing Chinese SRT: %s (%s)\n", zhPath, fileSizeStr(zhPath))
			return result, nil
		}
		// Validation failed — likely because source SRT was re-generated with
		// sentence breaking. Invalidate the stale translation and re-translate.
		_ = os.Remove(zhPath)
	}

	if opts.Translator == nil {
		return nil, fmt.Errorf("subtitle translator is required")
	}

	// Stage: Translation
	start := time.Now()
	fmt.Fprintf(os.Stderr, "Translating to Chinese... ")
	sourceData, err := os.ReadFile(srtPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED\n")
		return nil, err
	}
	translated, err := opts.Translator.TranslateSRT(ctx, string(sourceData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED\n")
		return nil, err
	}
	if err := validateTranslatedSRT(string(sourceData), translated); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED\n")
		return nil, err
	}
	if err := os.WriteFile(zhPath, []byte(translated), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED\n")
		return nil, err
	}
	entries, _ := srt.CountBlocks(zhPath)
	fmt.Fprintf(os.Stderr, "done (%v) — %d entries, %s\n", time.Since(start).Round(time.Millisecond), entries, fileSizeStr(zhPath))

	return result, nil
}

func subtitlePath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".srt"
}

// subtitledVideoPath returns the path for soft-subtitled video (backward compatible)
func subtitledVideoPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".subtitled.mp4"
}

// subtitledVideoPathForMode returns the path for subtitled video in the given mode
func subtitledVideoPathForMode(videoPath string, mode Mode) string {
	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	if mode == ModeSoft {
		return base + ".subtitled.mp4"
	}
	return base + ".burned.mp4"
}

func chineseSubtitlePath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".zh.srt"
}

// chineseSubtitledVideoPath returns the path for soft-subtitled Chinese video (backward compatible)
func chineseSubtitledVideoPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".zh.subtitled.mp4"
}

// chineseSubtitledVideoPathForMode returns the path for subtitled Chinese video in the given mode
func chineseSubtitledVideoPathForMode(videoPath string, mode Mode) string {
	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	if mode == ModeSoft {
		return base + ".zh.subtitled.mp4"
	}
	return base + ".zh.burned.mp4"
}
