// yt-2-bili is a command-line tool that downloads YouTube videos and uploads them to Bilibili.
package main

import (
	"fmt"
	"os"

	"github.com/dannis/yt-2-bili/internal/workflow"
	"github.com/spf13/cobra"
)

var (
	cookie    string
	outputDir string
	quality   string
	tid       int
	keepVideo bool
	ytDlpPath string
	biliupPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "yt-2-bili <youtube-url>",
		Short: "Download YouTube videos and upload to Bilibili",
		Long: `yt-2-bili downloads YouTube videos using yt-dlp and uploads them to Bilibili using biliup-rs.

Requires:
  - yt-dlp: https://github.com/yt-dlp/yt-dlp
  - biliup-rs: https://github.com/biliup/biliup-rs

Example:
  yt-2-bili --cookie cookies.json https://www.youtube.com/watch?v=dQw4w9WgXcQ`,
		Args: cobra.ExactArgs(1),
		RunE: run,
	}

	rootCmd.Flags().StringVarP(&cookie, "cookie", "c", "", "Path to biliup cookies.json (default: cookies.json in current directory)")
	rootCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Directory to save downloaded files (default: temp directory)")
	rootCmd.Flags().StringVarP(&quality, "quality", "q", "1080p", "Video quality (1080p, 720p, 480p, best)")
	rootCmd.Flags().IntVarP(&tid, "tid", "t", 171, "Bilibili投稿分区 (default: 171 游戏区)")
	rootCmd.Flags().BoolVar(&keepVideo, "keep-video", false, "Keep downloaded video files after upload (default: delete)")
	rootCmd.Flags().StringVar(&ytDlpPath, "yt-dlp-path", "", "Path to yt-dlp executable (default: look in PATH)")
	rootCmd.Flags().StringVar(&biliupPath, "biliup-path", "", "Path to biliup executable (default: look in PATH)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Default cookie path
	if cookie == "" {
		if _, err := os.Stat("cookies.json"); err == nil {
			cookie = "cookies.json"
		}
	}

	// Default output dir
	if outputDir == "" {
		outputDir = workflow.DefaultOutputDir()
	}

	opts := workflow.Options{
		YouTubeURL:   args[0],
		BiliupCookie: cookie,
		OutputDir:    outputDir,
		Quality:      quality,
		Tid:          tid,
		KeepVideo:    keepVideo,
		YtDlpPath:    ytDlpPath,
		BiliupPath:   biliupPath,
		ShowProgress: true,
	}

	if err := workflow.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	return nil
}
