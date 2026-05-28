// Package ffmpeg provides operations for working with ffmpeg.
package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

// ConvertCover converts a cover image to JPEG if needed.
// If the input is already JPEG, returns the path unchanged.
func ConvertCover(path string) string {
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".webp") {
		return path
	}

	jpgPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".jpg"
	var stderr strings.Builder
	err := ffmpeg_go.Input(path).
		Output(jpgPath, ffmpeg_go.KwArgs{
			"frames:v": "1",
			"update":   "1",
		}).
		OverWriteOutput().
		WithErrorOutput(&stderr).
		Run()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to convert webp cover to jpg, using original cover: %v\nstderr: %s\n", err, stderr.String())
		return path
	}
	return jpgPath
}

// EmbedSoftSubtitle embeds an SRT as a soft subtitle track in a video.
func EmbedSoftSubtitle(videoPath, srtPath, outputPath string) error {
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

// BurnSubtitle burns an SRT directly into the video frames.
func BurnSubtitle(videoPath, srtPath, outputPath string) error {
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
			"map":    "0:a",
		}).
		OverWriteOutput().
		WithErrorOutput(&stderr).
		Run()

	if err != nil {
		return fmt.Errorf("ffmpeg subtitle burning failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

// HasSubtitleStream checks if a video has a subtitle stream using ffprobe.
func HasSubtitleStream(videoPath string) bool {
	if !isNonEmptyFile(videoPath) {
		return false
	}
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "s", "-show_entries", "stream=index", "-of", "csv=p=0", videoPath)
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) != ""
}

// CheckAvailable verifies that ffmpeg and ffprobe are available.
func CheckAvailable() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found: %w", err)
	}
	return nil
}

func escapeFFmpegPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "\\\\")
	path = strings.ReplaceAll(path, ":", "\\:")
	path = strings.ReplaceAll(path, "'", "\\'")
	return path
}

func getFontName() string {
	switch runtime.GOOS {
	case "windows":
		return "Microsoft YaHei"
	case "darwin":
		return "PingFang SC"
	default:
		return "Noto Sans CJK SC"
	}
}

func isNonEmptyFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Size() > 0 && !stat.IsDir()
}
