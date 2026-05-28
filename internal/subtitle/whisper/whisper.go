// Package whisper provides integration with the whisper CLI.
package whisper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options for generating SRT with whisper.
type Options struct {
	VideoPath          string
	WhisperPath        string
	ModelDirectory     string
	WhisperDevice      string
	WhisperComputeType string
}

// GenerateSRT generates an SRT file using whisper.
func GenerateSRT(opts Options, outputPath string) error {
	args := buildArgs(opts)
	cmd := exec.Command(resolvePath(opts.WhisperPath), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("whisper subtitle generation failed: %w\nstderr: %s", err, stderr.String())
	}
	if !isNonEmptyFile(outputPath) {
		return fmt.Errorf("expected whisper output was not found: %s", outputPath)
	}
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

func isNonEmptyFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Size() > 0 && !stat.IsDir()
}
