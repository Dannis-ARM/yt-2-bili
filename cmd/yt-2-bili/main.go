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
	forceSubtitles bool
	ytDlpPath     string
	biliupPath    string

	generateSubtitles      bool
	subtitleModeStr        string
	whisperPath            string
	whisperModelDirectory  string
	whisperDevice          string
	whisperComputeType     string
	subtitleTargetLanguage string
	llmModelName           string

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
	rootCmd.AddCommand(newDownloadCmd(), newUploadCmd(), newTransferCmd(), newSubtitleCmd())

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

func newSubtitleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subtitle <video-file>",
		Short: "Generate subtitles for a local video file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prepareDefaults()
			return runSubtitle(args[0])
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
	cmd.PersistentFlags().BoolVar(&forceSubtitles, "force-subtitles", false, "Force regenerate subtitles even if files already exist")
	cmd.PersistentFlags().BoolVar(&generateSubtitles, "generate-subtitles", false, "Generate SRT subtitles and embed into MP4 (burned by default)")
	cmd.PersistentFlags().StringVar(&subtitleModeStr, "subtitle-mode", "", "Subtitle mode: burned/hard (default, burned into video) or soft (embedded track)")
	cmd.PersistentFlags().StringVar(&whisperPath, "whisper-path", "", "Path to whisper executable (default: look in PATH)")
	cmd.PersistentFlags().StringVar(&whisperModelDirectory, "whisper-model-directory", "", "Whisper model directory; passed as --model_directory to compatible CLIs")
	cmd.PersistentFlags().StringVar(&whisperDevice, "whisper-device", "", "Whisper device (auto, cpu, cuda); passed as --device to compatible CLIs")
	cmd.PersistentFlags().StringVar(&whisperComputeType, "whisper-compute-type", "", "Whisper compute type (int8, float16, float32); overrides default int8")
	cmd.PersistentFlags().StringVar(&subtitleTargetLanguage, "subtitle-target-language", "", "Target language for subtitle translation (e.g. zh); requires --generate-subtitles")
	cmd.PersistentFlags().StringVar(&llmModelName, "llm-model-name", "deepseek-v4-flash", "LLM model name for subtitle translation")
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
	forceSubtitles = false
	ytDlpPath = ""
	biliupPath = ""
	generateSubtitles = false
	subtitleModeStr = ""
	whisperPath = ""
	whisperModelDirectory = ""
	whisperDevice = ""
	whisperComputeType = ""
	subtitleTargetLanguage = ""
	llmModelName = "deepseek-v4-flash"
	uploadTitle = ""
	uploadDesc = ""
	uploadCover = ""
	uploadSource = ""
	uploadTags = ""
}

func parseSubtitleMode() (subtitle.Mode, error) {
	return subtitle.ParseMode(subtitleModeStr)
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

func validateFlags(isSubtitleCommand bool) error {
	if !isSubtitleCommand && !generateSubtitles {
		// Check that whisper-related flags aren't used without --generate-subtitles
		if whisperPath != "" {
			return fmt.Errorf("--whisper-path requires --generate-subtitles")
		}
		if whisperModelDirectory != "" {
			return fmt.Errorf("--whisper-model-directory requires --generate-subtitles")
		}
		if whisperDevice != "" {
			return fmt.Errorf("--whisper-device requires --generate-subtitles")
		}
		if whisperComputeType != "" {
			return fmt.Errorf("--whisper-compute-type requires --generate-subtitles")
		}
		if subtitleTargetLanguage != "" {
			return fmt.Errorf("--subtitle-target-language requires --generate-subtitles")
		}
		if llmModelName != "deepseek-v4-flash" {
			return fmt.Errorf("--llm-model-name requires --generate-subtitles")
		}
		if subtitleModeStr != "" {
			return fmt.Errorf("--subtitle-mode requires --generate-subtitles")
		}
	}
	if subtitleTargetLanguage != "" && !generateSubtitles && !isSubtitleCommand {
		return fmt.Errorf("--subtitle-target-language requires --generate-subtitles")
	}
	// Validate subtitle mode if specified
	if subtitleModeStr != "" {
		if _, err := parseSubtitleMode(); err != nil {
			return err
		}
	}
	return nil
}

func runDownload(youtubeURL string) error {
	if err := validateFlags(false); err != nil {
		return err
	}
	if err := ytdlp.CheckAvailable(ytDlpPath); err != nil {
		return fmt.Errorf("dependency check failed: %w\nPlease install yt-dlp first", err)
	}

	fmt.Fprintln(os.Stderr, "Fetching video metadata...")
	info, err := ytdlp.GetVideoInfo(youtubeURL, ytDlpPath)
	if err != nil {
		return err
	}
	if strings.Contains(youtubeURL, "playlist") {
		return fmt.Errorf("playlist URLs are not supported yet")
	}

	fmt.Fprintf(os.Stderr, "Video found: %s (by %s)\n", info.Title, info.Uploader)
	fmt.Fprintln(os.Stderr, "Downloading video...")
	result, err := ytdlp.DownloadVideo(youtubeURL, ytdlp.DownloadOptions{
		OutputDir:     outputDir,
		Quality:       quality,
		CustomPath:    ytDlpPath,
		ForceDownload: forceDownload,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Downloaded video: %s\n", result.VideoPath)
	if result.ThumbnailPath != "" {
		fmt.Fprintf(os.Stderr, "Downloaded thumbnail: %s\n", result.ThumbnailPath)
	}
	if generateSubtitles {
		subtitleResult, err := ensureSubtitles(result.VideoPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Generated subtitle: %s\n", subtitleResult.SubtitlePath)
		modeText := "burned"
		if subtitleResult.SubtitleMode == subtitle.ModeSoft {
			modeText = "soft"
		}
		fmt.Fprintf(os.Stderr, "Generated subtitled video (%s): %s\n", modeText, subtitleResult.SubtitledVideoPath)
	}
	return nil
}

func runUpload(videoPath string) error {
	if err := validateFlags(false); err != nil {
		return err
	}
	if generateSubtitles {
		subtitleResult, err := ensureSubtitles(videoPath)
		if err != nil {
			return err
		}
		videoPath = subtitleResult.SubtitledVideoPath
		fmt.Fprintf(os.Stderr, "Using subtitled video: %s\n", videoPath)
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
	})
}

func ensureSubtitles(videoPath string) (*subtitle.Result, error) {
	fmt.Fprintln(os.Stderr, "Generating subtitles...")
	translator, err := makeTranslator()
	if err != nil {
		return nil, err
	}
	mode, err := parseSubtitleMode()
	if err != nil {
		return nil, err
	}
	return subtitle.EnsureSubtitled(subtitle.Options{
		VideoPath:              videoPath,
		WhisperPath:            whisperPath,
		ModelDirectory:         whisperModelDirectory,
		WhisperDevice:          whisperDevice,
		WhisperComputeType:     whisperComputeType,
		SubtitleTargetLanguage: subtitleTargetLanguage,
		SubtitleMode:           mode,
		Translator:             translator,
		Force:                  forceSubtitles,
	})
}

func makeTranslator() (subtitle.Translator, error) {
	if subtitleTargetLanguage == "" {
		return nil, nil
	}

	// Anthropic-compatible provider (e.g. DeepSeek)
	if token := os.Getenv("ANTHROPIC_AUTH_TOKEN"); token != "" {
		baseURL := os.Getenv("ANTHROPIC_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/anthropic"
		}
		return subtitle.NewLLMTranslator(subtitle.LLMTranslatorOptions{
			BaseURL:  baseURL,
			APIKey:   token,
			Model:    llmModelName,
			Provider: "anthropic",
		}), nil
	}

	// OpenAI-compatible provider (default: Volcengine Ark)
	apiKey := os.Getenv("ARK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ARK_API_KEY or ANTHROPIC_AUTH_TOKEN environment variable is required for subtitle translation")
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/coding/v3"
	}
	return subtitle.NewLLMTranslator(subtitle.LLMTranslatorOptions{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   llmModelName,
	}), nil
}

func runTransfer(youtubeURL string) error {
	if err := validateFlags(false); err != nil {
		return err
	}
	translator, err := makeTranslator()
	if err != nil {
		return err
	}
	subtitleMode, err := parseSubtitleMode()
	if err != nil {
		return err
	}
	opts := workflow.Options{
		YouTubeURL:             youtubeURL,
		BiliupCookie:           cookie,
		OutputDir:              outputDir,
		Quality:                quality,
		Tid:                    tid,
		Cleanup:                cleanup,
		ForceDownload:          forceDownload,
		ForceSubtitles:         forceSubtitles,
		GenerateSubtitles:      generateSubtitles,
		WhisperPath:            whisperPath,
		WhisperModelDirectory:  whisperModelDirectory,
		WhisperDevice:          whisperDevice,
		WhisperComputeType:     whisperComputeType,
		SubtitleTargetLanguage: subtitleTargetLanguage,
		SubtitleMode:           subtitleMode,
		Translator:             translator,
		YtDlpPath:              ytDlpPath,
		BiliupPath:             biliupPath,
	}

	return workflow.Run(opts)
}

func runSubtitle(videoPath string) error {
	if err := validateFlags(true); err != nil {
		return err
	}
	if err := subtitle.CheckAvailable(whisperPath); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	subtitleResult, err := ensureSubtitles(videoPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Generated subtitle: %s\n", subtitleResult.SubtitlePath)
	if subtitleResult.ChineseSubtitlePath != "" {
		fmt.Fprintf(os.Stderr, "Generated Chinese subtitle: %s\n", subtitleResult.ChineseSubtitlePath)
	}
	modeText := "burned"
	if subtitleResult.SubtitleMode == subtitle.ModeSoft {
		modeText = "soft"
	}
	fmt.Fprintf(os.Stderr, "Generated subtitled video (%s): %s\n", modeText, subtitleResult.SubtitledVideoPath)
	return nil
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
