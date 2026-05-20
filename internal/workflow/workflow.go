// Package workflow coordinates the download and upload process.
package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dannis/yt-2-bili/internal/biliup"
	"github.com/dannis/yt-2-bili/internal/ytdlp"
)

// Options contains all options for running the workflow.
type Options struct {
	YouTubeURL      string
	BiliupCookie    string
	OutputDir       string
	Quality         string
	Tid             int
	KeepVideo       bool
	YtDlpPath       string
	BiliupPath      string
	ShowProgress    bool
}

// Run executes the full workflow: download YouTube video, then upload to Bilibili.
func Run(opts Options) error {
	// Check dependencies first
	if err := ytdlp.CheckAvailable(opts.YtDlpPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install yt-dlp first", err)
	}
	if err := biliup.CheckAvailable(opts.BiliupPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install biliup-rs first", err)
	}

	// Step 1: Get video info first
	fmt.Println("Fetching video metadata...")
	info, err := ytdlp.GetVideoInfo(opts.YouTubeURL, opts.YtDlpPath)
	if err != nil {
		return err
	}

	// Check if it's a playlist URL but the user didn't mean to
	if strings.Contains(opts.YouTubeURL, "playlist") {
		return fmt.Errorf("playlist URLs are not supported yet")
	}

	fmt.Printf("Video found: %s (by %s)\n", info.Title, info.Uploader)

	// Step 2: Download video and thumbnail
	fmt.Println("Downloading video...")
	downloadOpts := ytdlp.DownloadOptions{
		OutputDir:    opts.OutputDir,
		Quality:      opts.Quality,
		CustomPath:   opts.YtDlpPath,
		ShowProgress: opts.ShowProgress,
	}
	result, err := ytdlp.DownloadVideo(opts.YouTubeURL, downloadOpts)
	if err != nil {
		return err
	}
	fmt.Printf("Downloaded to: %s\n", result.VideoPath)

	// Cleanup function (deferred but only runs if KeepVideo is false)
	cleanup := func() {
		if opts.KeepVideo {
			fmt.Println("Keeping downloaded files (--keep-video specified)")
			return
		}
		fmt.Println("Cleaning up downloaded files...")
		if result.VideoPath != "" {
			_ = os.Remove(result.VideoPath)
		}
		if result.ThumbnailPath != "" {
			_ = os.Remove(result.ThumbnailPath)
		}
		// Try to remove the directory if it's empty
		if opts.OutputDir != "" {
			_ = os.Remove(opts.OutputDir)
		}
	}

	// Step 3: Build Bilibili description
	bilibiliDesc := buildBilibiliDesc(info.Description, info.Uploader, info.WebpageURL)

	// Step 4: Upload to Bilibili
	fmt.Println("Uploading to Bilibili...")
	uploadOpts := biliup.UploadOptions{
		VideoPath:      result.VideoPath,
		CoverPath:      result.ThumbnailPath,
		Title:          info.Title,
		Desc:           bilibiliDesc,
		Source:         info.WebpageURL,
		Tags:           info.Tags,
		Tid:            opts.Tid,
		Copyright:      2, // Always reupload
		UserCookiePath: opts.BiliupCookie,
		CustomPath:     opts.BiliupPath,
		ShowProgress:   opts.ShowProgress,
	}

	// Run upload
	err = biliup.Upload(uploadOpts)
	if err != nil {
		// Upload failed - keep the files so user can retry manually
		fmt.Printf("\nUpload failed! Downloaded files are kept at:\n")
		fmt.Printf("  Video: %s\n", result.VideoPath)
		if result.ThumbnailPath != "" {
			fmt.Printf("  Thumbnail: %s\n", result.ThumbnailPath)
		}
		return err
	}

	// Success - cleanup unless --keep-video is specified
	cleanup()

	fmt.Println("\nDone! Video uploaded successfully.")
	return nil
}

// buildBilibiliDesc builds the Bilibili description with original author and link.
func buildBilibiliDesc(originalDesc, uploader, youtubeURL string) string {
	var sb strings.Builder

	// Add attribution at the top
	sb.WriteString(fmt.Sprintf("原作者: %s\n", uploader))
	sb.WriteString(fmt.Sprintf("原视频链接: %s\n", youtubeURL))
	sb.WriteString("====================\n\n")

	// Add original description
	sb.WriteString(originalDesc)

	return sb.String()
}

// DefaultOutputDir returns the default output directory.
func DefaultOutputDir() string {
	// Use a temp directory with a unique name
	return filepath.Join(os.TempDir(), "yt-2-bili")
}
