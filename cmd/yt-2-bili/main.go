package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dannis/yt-2-bili/internal/biliup"
	"github.com/dannis/yt-2-bili/internal/subtitle"
	"github.com/dannis/yt-2-bili/internal/workflow"
	"github.com/dannis/yt-2-bili/internal/ytdlp"
	"github.com/spf13/cobra"
)

var (
	cookie        string
	outputDir     string
	quality       string
	tid           int
	cleanup       bool
	forceDownload bool
	ytDlpPath     string
	biliupPath    string

	generateSubtitles     bool
	whisperPath           string
	whisperModelDirectory string

	uploadTitle  string
	uploadDesc   string
	uploadCover  string
	uploadSource string
	uploadTags   string
)

func main() {
	rootCmd := newRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	resetFlags()

	rootCmd := &cobra.Command{
		Use:           "yt-2-bili <command>",
		Short:         "Download YouTube videos and upload to Bilibili",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `yt-2-bili downloads YouTube videos using yt-dlp and uploads them to Bilibili using biliup-rs.

Requires:
  - yt-dlp: https://github.com/yt-dlp/yt-dlp
  - biliup-rs: https://github.com/biliup/biliup-rs

Examples:
  yt-2-bili download https://www.youtube.com/watch?v=dQw4w9WgXcQ
  yt-2-bili upload --cookie cookies.json --title "Video title" video.mp4
  yt-2-bili transfer --cookie cookies.json https://www.youtube.com/watch?v=dQw4w9WgXcQ`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	addCommonFlags(rootCmd)
	rootCmd.AddCommand(newDownloadCmd(), newUploadCmd(), newTransferCmd())

	return rootCmd
}

func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <youtube-url>",
		Short: "Download a YouTube video and thumbnail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prepareDefaults()
			return runDownload(args[0])
		},
	}
	return cmd
}

func newUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <video-path>",
		Short: "Upload a local video to Bilibili",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prepareDefaults()
			return runUpload(args[0])
		},
	}
	cmd.Flags().StringVar(&uploadTitle, "title", "", "Bilibili video title")
	cmd.Flags().StringVar(&uploadDesc, "desc", "", "Bilibili video description")
	cmd.Flags().StringVar(&uploadCover, "cover", "", "Bilibili video cover path")
	cmd.Flags().StringVar(&uploadSource, "source", "", "Original video source URL")
	cmd.Flags().StringVar(&uploadTags, "tag", "", "Comma-separated Bilibili tags")
	return cmd
}

func newTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer <youtube-url>",
		Short: "Download a YouTube video and upload it to Bilibili",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prepareDefaults()
			return runTransfer(args[0])
		},
	}
	return cmd
}

func addCommonFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&cookie, "cookie", "c", "", "Path to biliup cookies.json (default: cookies.json in current directory)")
	cmd.PersistentFlags().StringVarP(&outputDir, "output-dir", "o", "", fmt.Sprintf("Directory to save downloaded files (default: %s)", workflow.DefaultOutputDir()))
	cmd.PersistentFlags().StringVarP(&quality, "quality", "q", "1080p", "Video quality (1080p, 720p, 480p, best)")
	cmd.PersistentFlags().IntVarP(&tid, "tid", "t", 171, "Bilibili投稿分区 (default: 171 游戏区)")
	cmd.PersistentFlags().BoolVar(&cleanup, "cleanup", false, "Clean up generated files after a successful transfer")
	cmd.PersistentFlags().BoolVar(&forceDownload, "force-download", false, "Download again even if the expected video file already exists")
	cmd.PersistentFlags().BoolVar(&generateSubtitles, "generate-subtitles", false, "Generate SRT subtitles and embed them as soft subtitles into an MP4")
	cmd.PersistentFlags().StringVar(&whisperPath, "whisper-path", "", "Path to whisper executable (default: look in PATH)")
	cmd.PersistentFlags().StringVar(&whisperModelDirectory, "whisper-model-directory", "", "Whisper model directory; passed as --model_directory to compatible CLIs")
	cmd.PersistentFlags().StringVar(&ytDlpPath, "yt-dlp-path", "", "Path to yt-dlp executable (default: look in PATH)")
	cmd.PersistentFlags().StringVar(&biliupPath, "biliup-path", "", "Path to biliup executable (default: look in PATH)")
}

func resetFlags() {
	cookie = ""
	outputDir = ""
	quality = "1080p"
	tid = 171
	cleanup = false
	forceDownload = false
	ytDlpPath = ""
	biliupPath = ""
	generateSubtitles = false
	whisperPath = ""
	whisperModelDirectory = ""
	uploadTitle = ""
	uploadDesc = ""
	uploadCover = ""
	uploadSource = ""
	uploadTags = ""
}

func prepareDefaults() {
	if cookie == "" {
		if _, err := os.Stat("cookies.json"); err == nil {
			cookie = "cookies.json"
		}
	}
	if outputDir == "" {
		outputDir = workflow.DefaultOutputDir()
	}
}

func runDownload(youtubeURL string) error {
	if err := ytdlp.CheckAvailable(ytDlpPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install yt-dlp first", err)
	}

	fmt.Println("Fetching video metadata...")
	info, err := ytdlp.GetVideoInfo(youtubeURL, ytDlpPath)
	if err != nil {
		return err
	}
	if strings.Contains(youtubeURL, "playlist") {
		return fmt.Errorf("playlist URLs are not supported yet")
	}

	fmt.Printf("Video found: %s (by %s)\n", info.Title, info.Uploader)
	fmt.Println("Downloading video...")
	result, err := ytdlp.DownloadVideo(youtubeURL, ytdlp.DownloadOptions{
		OutputDir:     outputDir,
		Quality:       quality,
		CustomPath:    ytDlpPath,
		ShowProgress:  true,
		ForceDownload: forceDownload,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded video: %s\n", result.VideoPath)
	if result.ThumbnailPath != "" {
		fmt.Printf("Downloaded thumbnail: %s\n", result.ThumbnailPath)
	}
	if generateSubtitles {
		subtitleResult, err := ensureSubtitles(result.VideoPath)
		if err != nil {
			return err
		}
		fmt.Printf("Generated subtitle: %s\n", subtitleResult.SubtitlePath)
		fmt.Printf("Generated subtitled video: %s\n", subtitleResult.SubtitledVideoPath)
	}
	return nil
}

func runUpload(videoPath string) error {
	if generateSubtitles {
		subtitleResult, err := ensureSubtitles(videoPath)
		if err != nil {
			return err
		}
		videoPath = subtitleResult.SubtitledVideoPath
		fmt.Printf("Using subtitled video: %s\n", videoPath)
	}

	if uploadTitle == "" {
		return fmt.Errorf("--title is required for upload")
	}
	if err := biliup.CheckAvailable(biliupPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install biliup-rs first", err)
	}

	return biliup.Upload(biliup.UploadOptions{
		VideoPath:      videoPath,
		CoverPath:      uploadCover,
		Title:          uploadTitle,
		Desc:           uploadDesc,
		Source:         uploadSource,
		Tags:           splitTags(uploadTags),
		Tid:            tid,
		Copyright:      2,
		UserCookiePath: cookie,
		CustomPath:     biliupPath,
		ShowProgress:   true,
	})
}

func ensureSubtitles(videoPath string) (*subtitle.Result, error) {
	fmt.Println("Generating subtitles...")
	return subtitle.EnsureSoftSubtitled(subtitle.Options{
		VideoPath:      videoPath,
		WhisperPath:    whisperPath,
		ModelDirectory: whisperModelDirectory,
		ShowProgress:   true,
	})
}

func runTransfer(youtubeURL string) error {
	opts := workflow.Options{
		YouTubeURL:            youtubeURL,
		BiliupCookie:          cookie,
		OutputDir:             outputDir,
		Quality:               quality,
		Tid:                   tid,
		Cleanup:               cleanup,
		ForceDownload:         forceDownload,
		GenerateSubtitles:     generateSubtitles,
		WhisperPath:           whisperPath,
		WhisperModelDirectory: whisperModelDirectory,
		YtDlpPath:             ytDlpPath,
		BiliupPath:            biliupPath,
		ShowProgress:          true,
	}

	return workflow.Run(opts)
}

func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}

	parts := strings.Split(tags, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
