package subtitle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Options struct {
	VideoPath              string
	WhisperPath            string
	ModelDirectory         string
	SubtitleTargetLanguage string
	Translator             Translator
	ShowProgress           bool
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

func EnsureSoftSubtitled(opts Options) (*Result, error) {
	if opts.VideoPath == "" {
		return nil, fmt.Errorf("video path is required")
	}
	if err := CheckAvailable(opts.WhisperPath); err != nil {
		return nil, err
	}

	result, err := prepareSubtitleFiles(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	if hasSubtitleStream(result.SubtitledVideoPath) && opts.SubtitleTargetLanguage == "" {
		result.ReusedSubtitled = true
		return result, nil
	}

	subtitleForEmbedding := result.SubtitlePath
	if result.ChineseSubtitlePath != "" {
		subtitleForEmbedding = result.ChineseSubtitlePath
	}
	if err := embedSoftSubtitle(opts.VideoPath, subtitleForEmbedding, result.SubtitledVideoPath, opts.ShowProgress); err != nil {
		return nil, err
	}
	if !hasSubtitleStream(result.SubtitledVideoPath) {
		return nil, fmt.Errorf("ffmpeg finished but output has no subtitle stream: %s", result.SubtitledVideoPath)
	}

	return result, nil
}

func prepareSubtitleFiles(ctx context.Context, opts Options) (*Result, error) {
	srtPath := subtitlePath(opts.VideoPath)
	result := &Result{
		OriginalVideoPath:  opts.VideoPath,
		SubtitlePath:       srtPath,
		SubtitledVideoPath: subtitledVideoPath(opts.VideoPath),
	}

	reusedSubtitle := isNonEmptyFile(srtPath)
	if !reusedSubtitle {
		if err := generateSRT(opts, srtPath); err != nil {
			return nil, err
		}
		if !isNonEmptyFile(srtPath) {
			return nil, fmt.Errorf("whisper finished but subtitle file was not created: %s", srtPath)
		}
	}
	result.ReusedSubtitle = reusedSubtitle
	result.GeneratedSourceSubtitle = !reusedSubtitle

	if opts.SubtitleTargetLanguage == "" {
		return result, nil
	}
	if opts.SubtitleTargetLanguage != "zh" {
		return nil, fmt.Errorf("unsupported subtitle target language: %s", opts.SubtitleTargetLanguage)
	}

	zhPath := chineseSubtitlePath(opts.VideoPath)
	result.ChineseSubtitlePath = zhPath
	result.SubtitledVideoPath = chineseSubtitledVideoPath(opts.VideoPath)
	if isNonEmptyFile(zhPath) {
		sourceData, err := os.ReadFile(srtPath)
		if err != nil {
			return nil, err
		}
		translatedData, err := os.ReadFile(zhPath)
		if err != nil {
			return nil, err
		}
		if err := validateTranslatedSRT(string(sourceData), string(translatedData)); err != nil {
			return nil, err
		}
		result.ReusedChineseSubtitle = true
		return result, nil
	}

	if opts.Translator == nil {
		return nil, fmt.Errorf("subtitle translator is required")
	}
	sourceData, err := os.ReadFile(srtPath)
	if err != nil {
		return nil, err
	}
	translated, err := opts.Translator.TranslateSRT(ctx, string(sourceData))
	if err != nil {
		return nil, err
	}
	if err := validateTranslatedSRT(string(sourceData), translated); err != nil {
		return nil, err
	}
	if err := os.WriteFile(zhPath, []byte(translated), 0o644); err != nil {
		return nil, err
	}
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

func subtitledVideoPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".subtitled.mp4"
}

func chineseSubtitlePath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".zh.srt"
}

func chineseSubtitledVideoPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".zh.subtitled.mp4"
}

func generateSRT(opts Options, expectedSRTPath string) error {
	args := buildWhisperArgs(opts)
	cmd := exec.Command(resolveWhisperPath(opts.WhisperPath), args...)
	if opts.ShowProgress {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("whisper subtitle generation failed: %w", err)
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
	}
	if opts.ModelDirectory != "" {
		args = append(args, "--model_directory", opts.ModelDirectory)
	}
	return args
}

func embedSoftSubtitle(videoPath, srtPath, outputPath string, showProgress bool) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", srtPath,
		"-c:v", "copy",
		"-c:a", "copy",
		"-c:s", "mov_text",
		outputPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	if showProgress {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg soft subtitle embedding failed: %w", err)
	}
	return nil
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
