// Package workflow coordinates the download and upload process.
package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dannis/yt-2-bili/internal/biliup"
	"github.com/dannis/yt-2-bili/internal/subtitle"
	"github.com/dannis/yt-2-bili/internal/ytdlp"
)

// Options contains all options for running the workflow.
type Options struct {
	YouTubeURL             string
	BiliupCookie           string
	OutputDir              string
	Quality                string
	Tid                    int
	Cleanup                bool
	ForceDownload          bool
	GenerateSubtitles      bool
	WhisperPath            string
	WhisperModelDirectory  string
	SubtitleTargetLanguage string
	Translator             subtitle.Translator
	YtDlpPath              string
	BiliupPath             string
	ShowProgress           bool
}

// Run executes the full workflow: download YouTube video, then upload to Bilibili.
func Run(opts Options) error {
	if err := ytdlp.CheckAvailable(opts.YtDlpPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install yt-dlp first", err)
	}
	if err := biliup.CheckAvailable(opts.BiliupPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install biliup-rs first", err)
	}

	fmt.Println("Fetching video metadata...")
	info, err := ytdlp.GetVideoInfo(opts.YouTubeURL, opts.YtDlpPath)
	if err != nil {
		return err
	}

	if strings.Contains(opts.YouTubeURL, "playlist") {
		return fmt.Errorf("playlist URLs are not supported yet")
	}

	fmt.Printf("Video found: %s (by %s)\n", info.Title, info.Uploader)

	fmt.Println("Downloading video...")
	downloadOpts := ytdlp.DownloadOptions{
		OutputDir:     opts.OutputDir,
		Quality:       opts.Quality,
		CustomPath:    opts.YtDlpPath,
		ShowProgress:  opts.ShowProgress,
		ForceDownload: opts.ForceDownload,
	}
	result, err := ytdlp.DownloadVideo(opts.YouTubeURL, downloadOpts)
	if err != nil {
		return err
	}
	fmt.Printf("Downloaded to: %s\n", result.VideoPath)

	artifacts := []string{result.VideoPath, result.ThumbnailPath}
	uploadVideoPath := result.VideoPath
	if opts.GenerateSubtitles {
		fmt.Println("Generating subtitles...")
		subtitleResult, err := subtitle.EnsureSoftSubtitled(subtitle.Options{
			VideoPath:              result.VideoPath,
			WhisperPath:            opts.WhisperPath,
			ModelDirectory:         opts.WhisperModelDirectory,
			SubtitleTargetLanguage: opts.SubtitleTargetLanguage,
			Translator:             opts.Translator,
			ShowProgress:           opts.ShowProgress,
		})
		if err != nil {
			return err
		}
		uploadVideoPath = subtitleResult.SubtitledVideoPath
		artifacts = append(artifacts, subtitleResult.SubtitlePath, subtitleResult.ChineseSubtitlePath, subtitleResult.SubtitledVideoPath)
		fmt.Printf("Generated subtitle: %s\n", subtitleResult.SubtitlePath)
		if subtitleResult.ChineseSubtitlePath != "" {
			fmt.Printf("Generated Chinese subtitle: %s\n", subtitleResult.ChineseSubtitlePath)
		}
		fmt.Printf("Generated subtitled video: %s\n", subtitleResult.SubtitledVideoPath)
	}

	cleanup := func() {
		if !opts.Cleanup {
			fmt.Println("Keeping generated files (default)")
			return
		}
		fmt.Println("Cleaning up generated files...")
		for _, artifact := range artifacts {
			if artifact != "" {
				_ = os.Remove(artifact)
			}
		}
		if result.ThumbnailPath != "" {
			convertedCover := strings.TrimSuffix(result.ThumbnailPath, filepath.Ext(result.ThumbnailPath)) + ".jpg"
			if convertedCover != result.ThumbnailPath {
				_ = os.Remove(convertedCover)
			}
		}
		if opts.OutputDir != "" {
			_ = os.Remove(opts.OutputDir)
		}
	}

	bilibiliDesc := buildBilibiliDesc(info.Description, info.Uploader, info.WebpageURL)

	coverPath := prepareCoverForUpload(result.ThumbnailPath)
	fmt.Println("Uploading to Bilibili...")
	uploadOpts := biliup.UploadOptions{
		VideoPath:      uploadVideoPath,
		CoverPath:      coverPath,
		Title:          info.Title,
		Desc:           bilibiliDesc,
		Source:         info.WebpageURL,
		Tags:           info.Tags,
		Tid:            opts.Tid,
		Copyright:      2,
		UserCookiePath: opts.BiliupCookie,
		CustomPath:     opts.BiliupPath,
		ShowProgress:   opts.ShowProgress,
	}

	err = biliup.Upload(uploadOpts)
	if err != nil {
		fmt.Printf("\nUpload failed! Downloaded files are kept at:\n")
		fmt.Printf("  Video: %s\n", uploadVideoPath)
		if result.ThumbnailPath != "" {
			fmt.Printf("  Thumbnail: %s\n", result.ThumbnailPath)
		}
		return err
	}

	cleanup()

	fmt.Println("\nDone! Video uploaded successfully.")
	return nil
}

func prepareCoverForUpload(coverPath string) string {
	if coverPath == "" || !strings.EqualFold(filepath.Ext(coverPath), ".webp") {
		return coverPath
	}

	jpgPath := strings.TrimSuffix(coverPath, filepath.Ext(coverPath)) + ".jpg"
	cmd := exec.Command("ffmpeg", "-y", "-i", coverPath, "-frames:v", "1", "-update", "1", jpgPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to convert webp cover to jpg, using original cover: %v\n", err)
		return coverPath
	}
	return jpgPath
}

func buildBilibiliDesc(originalDesc, uploader, youtubeURL string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("原作者: %s\n", uploader))
	sb.WriteString(fmt.Sprintf("原视频链接: %s\n", youtubeURL))
	sb.WriteString("====================\n\n")

	sb.WriteString(originalDesc)

	return sb.String()
}

func DefaultOutputDir() string {
	return filepath.Join(os.TempDir(), "yt-2-bili")
}
