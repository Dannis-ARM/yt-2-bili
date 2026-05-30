// Package whisper provides integration with the whisper CLI.
package whisper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dannis/yt-2-bili/internal/subtitle/breaker"
)

// Options for generating SRT with whisper.
type Options struct {
	VideoPath          string
	WhisperPath        string
	ModelDirectory     string
	WhisperDevice      string
	WhisperComputeType string
}

// GenerateSRT generates an SRT file using whisper with word-level timestamps.
// It outputs JSON first, then converts to SRT using the breaker package.
func GenerateSRT(opts Options, outputPath string) error {
	args := buildArgs(opts)
	cmd := exec.Command(resolvePath(opts.WhisperPath), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("whisper subtitle generation failed: %w\nstderr: %s", err, stderr.String())
	}

	// Whisper outputs JSON to the same directory as the video, with .json extension
	jsonPath := jsonOutputPath(opts.VideoPath)
	if !isNonEmptyFile(jsonPath) {
		return fmt.Errorf("expected whisper JSON output not found: %s", jsonPath)
	}

	// Convert JSON to SRT using breaker
	if err := breaker.ProcessJSONFile(jsonPath, outputPath); err != nil {
		return fmt.Errorf("converting JSON to SRT: %w", err)
	}

	// Clean up the JSON file (optional but keeps things tidy)
	_ = os.Remove(jsonPath)

	return nil
}

// CheckAvailable checks if whisper is available.
func CheckAvailable(customPath string) error {
	if _, err := exec.LookPath(resolvePath(customPath)); err != nil {
		return fmt.Errorf("whisper not found: %w", err)
	}
	return nil
}

func resolvePath(customPath string) string {
	if customPath != "" {
		return customPath
	}
	return "whisper"
}

func buildArgs(opts Options) []string {
	args := []string{
		opts.VideoPath,
		"--output_format", "json",
		"--output_dir", filepath.Dir(opts.VideoPath),
		"--word_timestamps", "True",
		"--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
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

func jsonOutputPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	return base + ".json"
}

func isNonEmptyFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Size() > 0 && !stat.IsDir()
}
