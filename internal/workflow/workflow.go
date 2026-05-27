// Package workflow coordinates the download and upload process.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"

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
	ForceSubtitles         bool
	GenerateSubtitles      bool
	WhisperPath            string
	WhisperModelDirectory  string
	WhisperDevice          string
	WhisperComputeType     string
	SubtitleTargetLanguage string
	SubtitleMode           subtitle.Mode
	Translator             subtitle.Translator
	YtDlpPath              string
	BiliupPath             string
}

// Run executes the full workflow: download YouTube video, then upload to Bilibili.
func Run(opts Options) error {
	if err := ytdlp.CheckAvailable(opts.YtDlpPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install yt-dlp first", err)
	}
	if err := biliup.CheckAvailable(opts.BiliupPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install biliup-rs first", err)
	}

	fmt.Fprintln(os.Stderr, "Fetching video metadata...")
	info, err := ytdlp.GetVideoInfo(opts.YouTubeURL, opts.YtDlpPath)
	if err != nil {
		return err
	}

	if strings.Contains(opts.YouTubeURL, "playlist") {
		return fmt.Errorf("playlist URLs are not supported yet")
	}

	fmt.Fprintf(os.Stderr, "Video found: %s (by %s)\n", info.Title, info.Uploader)

	fmt.Fprintln(os.Stderr, "Downloading video...")
	downloadOpts := ytdlp.DownloadOptions{
		OutputDir:     opts.OutputDir,
		Quality:       opts.Quality,
		CustomPath:    opts.YtDlpPath,
		ForceDownload: opts.ForceDownload,
	}
	result, err := ytdlp.DownloadVideo(opts.YouTubeURL, downloadOpts)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Downloaded to: %s\n", result.VideoPath)

	artifacts := []string{result.VideoPath, result.ThumbnailPath}
	uploadVideoPath := result.VideoPath
	uploadTitle := info.Title
	uploadDesc := info.Description

	if opts.GenerateSubtitles && opts.Translator != nil && opts.SubtitleTargetLanguage != "" {
		fmt.Fprintln(os.Stderr, "Generating subtitles...")
		subtitleOpts := subtitle.Options{
			VideoPath:              result.VideoPath,
			WhisperPath:            opts.WhisperPath,
			ModelDirectory:         opts.WhisperModelDirectory,
			WhisperDevice:          opts.WhisperDevice,
			WhisperComputeType:     opts.WhisperComputeType,
			SubtitleTargetLanguage: opts.SubtitleTargetLanguage,
			SubtitleMode:           opts.SubtitleMode,
			Translator:             opts.Translator,
			Force:                  opts.ForceSubtitles,
		}
		subtitleResult, err := subtitle.EnsureSubtitled(subtitleOpts)
		if err != nil {
			return err
		}
		uploadVideoPath = subtitleResult.SubtitledVideoPath
		artifacts = append(artifacts, subtitleResult.SubtitlePath, subtitleResult.ChineseSubtitlePath, subtitleResult.SubtitledVideoPath)
		fmt.Fprintf(os.Stderr, "Generated subtitle: %s\n", subtitleResult.SubtitlePath)
		if subtitleResult.ChineseSubtitlePath != "" {
			fmt.Fprintf(os.Stderr, "Generated Chinese subtitle: %s\n", subtitleResult.ChineseSubtitlePath)
		}
		modeText := "burned"
		if subtitleResult.SubtitleMode == subtitle.ModeSoft {
			modeText = "soft"
		}
		fmt.Fprintf(os.Stderr, "Generated subtitled video (%s): %s\n", modeText, subtitleResult.SubtitledVideoPath)

		// Translate title and description in parallel
		ctx := context.Background()
		fmt.Fprintln(os.Stderr, "Translating title and description...")

		type translateResult struct {
			text string
			err  error
		}
		titleChan := make(chan translateResult, 1)
		descChan := make(chan translateResult, 1)

		go func() {
			text, err := opts.Translator.TranslateText(ctx, info.Title)
			titleChan <- translateResult{text: text, err: err}
		}()
		go func() {
			text, err := opts.Translator.TranslateText(ctx, info.Description)
			descChan <- translateResult{text: text, err: err}
		}()

		titleResult := <-titleChan
		if titleResult.err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to translate title, using original: %v\n", titleResult.err)
		} else {
			uploadTitle = fmt.Sprintf("[中字] %s | %s", titleResult.text, info.Title)
			fmt.Fprintf(os.Stderr, "Translated title: %s\n", uploadTitle)
		}

		descResult := <-descChan
		if descResult.err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to translate description, using original: %v\n", descResult.err)
		} else {
			uploadDesc = descResult.text
			fmt.Fprintf(os.Stderr, "Description translated successfully\n")
		}
	}

	cleanup := func() {
		if !opts.Cleanup {
			fmt.Fprintln(os.Stderr, "Keeping generated files (default)")
			return
		}
		fmt.Fprintln(os.Stderr, "Cleaning up generated files...")
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

	bilibiliDesc := buildBilibiliDesc(uploadDesc, info.Uploader, info.WebpageURL)

	coverPath := prepareCoverForUpload(result.ThumbnailPath)
	fmt.Fprintln(os.Stderr, "Uploading to Bilibili...")
	uploadOpts := biliup.UploadOptions{
		VideoPath:      uploadVideoPath,
		CoverPath:      coverPath,
		Title:          uploadTitle,
		Desc:           bilibiliDesc,
		Source:         info.WebpageURL,
		Tags:           info.Tags,
		Tid:            opts.Tid,
		Copyright:      2,
		UserCookiePath: opts.BiliupCookie,
		CustomPath:     opts.BiliupPath,
	}

	err = biliup.Upload(uploadOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nUpload failed! Downloaded files are kept at:\n")
		fmt.Fprintf(os.Stderr, "  Video: %s\n", uploadVideoPath)
		if result.ThumbnailPath != "" {
			fmt.Fprintf(os.Stderr, "  Thumbnail: %s\n", result.ThumbnailPath)
		}
		return err
	}

	cleanup()

	fmt.Fprintln(os.Stderr, "\nDone! Video uploaded successfully.")
	return nil
}

func prepareCoverForUpload(coverPath string) string {
	if coverPath == "" || !strings.EqualFold(filepath.Ext(coverPath), ".webp") {
		return coverPath
	}

	jpgPath := strings.TrimSuffix(coverPath, filepath.Ext(coverPath)) + ".jpg"
	var stderr strings.Builder
	err := ffmpeg_go.Input(coverPath).
		Output(jpgPath, ffmpeg_go.KwArgs{
			"frames:v": "1",
			"update":   "1",
		}).
		OverWriteOutput().
		WithErrorOutput(&stderr).
		Run()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to convert webp cover to jpg, using original cover: %v\nstderr: %s\n", err, stderr.String())
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
