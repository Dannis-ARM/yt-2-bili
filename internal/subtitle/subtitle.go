package subtitle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"
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
	if _, err := exec.LookPath(resolveWhisperPath(whisperPath)); err != nil {
		return fmt.Errorf("whisper not found: %w", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found: %w", err)
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
		if opts.SubtitleMode == ModeSoft && hasSubtitleStream(result.SubtitledVideoPath) {
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
		if err := embedSoftSubtitle(opts.VideoPath, subtitleForEmbedding, result.SubtitledVideoPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		if !hasSubtitleStream(result.SubtitledVideoPath) {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, fmt.Errorf("ffmpeg finished but output has no subtitle stream: %s", result.SubtitledVideoPath)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Burning subtitles with ffmpeg... ")
		if err := burnSubtitle(opts.VideoPath, subtitleForEmbedding, result.SubtitledVideoPath); err != nil {
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
		if err := generateSRT(opts, srtPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		if !isNonEmptyFile(srtPath) {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, fmt.Errorf("whisper finished but subtitle file was not created: %s", srtPath)
		}
		entries, _ := countSRTBlocks(srtPath)
		fmt.Fprintf(os.Stderr, "done (%v) — %d entries, %s\n", time.Since(start).Round(time.Millisecond), entries, fileSizeStr(srtPath))

		// Stage: Sentence breaking
		start = time.Now()
		entriesBefore, _ := countSRTBlocks(srtPath)
		fmt.Fprintf(os.Stderr, "Applying sentence breaking... ")
		if err := applySentenceBreaking(srtPath); err != nil {
			fmt.Fprintf(os.Stderr, "FAILED\n")
			return nil, err
		}
		entriesAfter, _ := countSRTBlocks(srtPath)
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
	entries, _ := countSRTBlocks(zhPath)
	fmt.Fprintf(os.Stderr, "done (%v) — %d entries, %s\n", time.Since(start).Round(time.Millisecond), entries, fileSizeStr(zhPath))

	return result, nil
}

func resolveWhisperPath(customPath string) string {
	if customPath != "" {
		return customPath
	}
	return "whisper"
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

func generateSRT(opts Options, expectedSRTPath string) error {
	args := buildWhisperArgs(opts)
	cmd := exec.Command(resolveWhisperPath(opts.WhisperPath), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("whisper subtitle generation failed: %w\nstderr: %s", err, stderr.String())
	}
	if !isNonEmptyFile(expectedSRTPath) {
		return fmt.Errorf("expected whisper output was not found: %s", expectedSRTPath)
	}
	return nil
}

func buildWhisperArgs(opts Options) []string {
	args := []string{
		opts.VideoPath,
		"--output_format", "srt",
		"--output_dir", filepath.Dir(opts.VideoPath),
		"--batched", "True",
		"--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--word_timestamps", "True",
		"--max_line_width", "42",
		"--max_line_count", "1",
	}
	if opts.WhisperComputeType != "" {
		args = append(args, "--compute_type", opts.WhisperComputeType)
	} else {
		args = append(args, "--compute_type", "int8")
	}
	if opts.ModelDirectory != "" {
		args = append(args, "--model_directory", opts.ModelDirectory)
	}
	if opts.WhisperDevice != "" {
		args = append(args, "--device", opts.WhisperDevice)
	}
	return args
}

func embedSoftSubtitle(videoPath, srtPath, outputPath string) error {
	videoStream := ffmpeg_go.Input(videoPath)
	subtitleStream := ffmpeg_go.Input(srtPath)

	var stderr strings.Builder
	err := ffmpeg_go.Output([]*ffmpeg_go.Stream{videoStream, subtitleStream}, outputPath,
		ffmpeg_go.KwArgs{
			"c:v": "copy",
			"c:a": "copy",
			"c:s": "mov_text",
		}).
		OverWriteOutput().
		WithErrorOutput(&stderr).
		Run()

	if err != nil {
		return fmt.Errorf("ffmpeg soft subtitle embedding failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func burnSubtitle(videoPath, srtPath, outputPath string) error {
	var stderr strings.Builder
	err := ffmpeg_go.Input(videoPath).
		Filter("subtitles", ffmpeg_go.Args{escapeFFmpegPath(srtPath)},
			ffmpeg_go.KwArgs{
				"force_style": fmt.Sprintf("FontName=%s,FontSize=18,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,BorderStyle=1,Outline=2,Shadow=1,MarginV=10",
					getFontName()),
			}).
		Output(outputPath, ffmpeg_go.KwArgs{
			"c:a":    "copy",
			"c:v":    "libx264",
			"crf":    "23",
			"preset": "medium",
		}).
		OverWriteOutput().
		WithErrorOutput(&stderr).
		Run()

	if err != nil {
		return fmt.Errorf("ffmpeg subtitle burning failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func escapeFFmpegPath(path string) string {
	// For ffmpeg's subtitles filter, need to escape colon, backslash, and single quote
	path = strings.ReplaceAll(path, "\\", "\\\\")
	path = strings.ReplaceAll(path, ":", "\\:")
	path = strings.ReplaceAll(path, "'", "\\'")
	return path
}

func getFontName() string {
	switch runtime.GOOS {
	case "windows":
		return "Microsoft YaHei" // 微软雅黑
	case "darwin":
		return "PingFang SC" // 苹方
	default:
		// Linux - try common CJK fonts
		return "Noto Sans CJK SC"
	}
}

func hasSubtitleStream(videoPath string) bool {
	if !isNonEmptyFile(videoPath) {
		return false
	}
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "s", "-show_entries", "stream=index", "-of", "csv=p=0", videoPath)
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) != ""
}

func isNonEmptyFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Size() > 0 && !stat.IsDir()
}

func countSRTBlocks(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	blocks, err := parseSRTBlocks(string(data))
	if err != nil {
		return 0, err
	}
	return len(blocks), nil
}

func fileSizeStr(path string) string {
	stat, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	size := stat.Size()
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
