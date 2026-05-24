// Package ytdlp provides functions to call yt-dlp for downloading YouTube videos and metadata.
package ytdlp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// VideoInfo represents the metadata returned by yt-dlp -J.
// Only the fields we need are included.
type VideoInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Uploader    string   `json:"uploader"`
	UploaderID  string   `json:"uploader_id"`
	WebpageURL  string   `json:"webpage_url"`
	Tags        []string `json:"tags"`
	Thumbnail   string   `json:"thumbnail"`
	Ext         string   `json:"ext"`
}

// CheckAvailable checks if yt-dlp is available in PATH or at the specified path.
func CheckAvailable(customPath string) error {
	path := "yt-dlp"
	if customPath != "" {
		path = customPath
	}
	_, err := exec.LookPath(path)
	if err != nil {
		return fmt.Errorf("yt-dlp not found: %w", err)
	}
	return nil
}

// GetVideoInfo retrieves video metadata using yt-dlp -J.
func GetVideoInfo(url string, customPath string) (*VideoInfo, error) {
	path := "yt-dlp"
	if customPath != "" {
		path = customPath
	}

	cmd := exec.Command(path, "-J", "--js-runtime", "bun", "--no-playlist", url)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var info VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %w", err)
	}

	return &info, nil
}

// DownloadOptions contains options for downloading a video.
type DownloadOptions struct {
	OutputDir    string
	Quality      string // "1080p", "720p", etc.
	CustomPath   string
	ShowProgress bool
}

// DownloadResult contains the paths to the downloaded files.
type DownloadResult struct {
	VideoPath     string
	ThumbnailPath string
}

// DownloadVideo downloads the video and thumbnail using yt-dlp.
func DownloadVideo(url string, opts DownloadOptions) (*DownloadResult, error) {
	path := "yt-dlp"
	if opts.CustomPath != "" {
		path = opts.CustomPath
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	outputTemplate := filepath.Join(opts.OutputDir, "%(id)s.%(ext)s")

	// Build format string based on quality
	format := "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]"
	if opts.Quality != "" && opts.Quality != "1080p" {
		switch opts.Quality {
		case "720p":
			format = "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[height<=720][ext=mp4]/best[height<=720]"
		case "480p":
			format = "bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/best[height<=480][ext=mp4]/best[height<=480]"
		case "best":
			format = "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best"
		}
	}

	args := []string{
		"-f", format,
		"--js-runtime", "bun",
		"--merge-output-format", "mp4",
		"--write-thumbnail",
		"-o", outputTemplate,
		"--no-playlist",
		url,
	}

	cmd := exec.Command(path, args...)

	// Show progress if requested
	if opts.ShowProgress {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp download failed: %w", err)
	}

	// First, get the info to know the actual filenames
	info, err := GetVideoInfo(url, opts.CustomPath)
	if err != nil {
		return nil, err
	}

	videoPath := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.mp4", info.ID))
	if _, err := os.Stat(videoPath); err != nil {
		videoPath = filepath.Join(opts.OutputDir, fmt.Sprintf("%s.%s", info.ID, info.Ext))
	}

	// Find the thumbnail file - yt-dlp saves it with the same name but different extension
	// We need to check common extensions
	thumbnailExts := []string{"jpg", "jpeg", "png", "webp"}
	var thumbnailPath string
	for _, ext := range thumbnailExts {
		candidate := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.%s", info.ID, ext))
		if _, err := os.Stat(candidate); err == nil {
			thumbnailPath = candidate
			break
		}
	}

	return &DownloadResult{
		VideoPath:     videoPath,
		ThumbnailPath: thumbnailPath,
	}, nil
}
