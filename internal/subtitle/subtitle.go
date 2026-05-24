package subtitle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Options struct {
	VideoPath    string
	WhisperPath  string
	Model        string
	Device       string
	Language     string
	Threads      int
	ShowProgress bool
}

type Result struct {
	OriginalVideoPath  string
	SubtitlePath       string
	SubtitledVideoPath string
	ReusedSubtitled    bool
	ReusedSubtitle     bool
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

	srtPath := subtitlePath(opts.VideoPath)
	subtitledPath := subtitledVideoPath(opts.VideoPath)
	if hasSubtitleStream(subtitledPath) {
		return &Result{
			OriginalVideoPath:  opts.VideoPath,
			SubtitlePath:       srtPath,
			SubtitledVideoPath: subtitledPath,
			ReusedSubtitled:    true,
		}, nil
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

	if err := embedSoftSubtitle(opts.VideoPath, srtPath, subtitledPath, opts.ShowProgress); err != nil {
		return nil, err
	}
	if !hasSubtitleStream(subtitledPath) {
		return nil, fmt.Errorf("ffmpeg finished but output has no subtitle stream: %s", subtitledPath)
	}

	return &Result{
		OriginalVideoPath:  opts.VideoPath,
		SubtitlePath:       srtPath,
		SubtitledVideoPath: subtitledPath,
		ReusedSubtitle:     reusedSubtitle,
	}, nil
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
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.Device != "" {
		args = append(args, "--device", opts.Device)
	}
	if opts.Language != "" {
		args = append(args, "--language", opts.Language)
	}
	if opts.Threads > 0 {
		args = append(args, "--threads", strconv.Itoa(opts.Threads))
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
