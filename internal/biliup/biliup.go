// Package biliup provides functions to call biliup-rs for uploading videos to Bilibili.
package biliup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CheckAvailable checks if biliup is available in PATH or at the specified path.
func CheckAvailable(customPath string) error {
	path := "biliup"
	if customPath != "" {
		path = customPath
	}
	_, err := exec.LookPath(path)
	if err != nil {
		return fmt.Errorf("biliup not found: %w", err)
	}
	return nil
}

// UploadOptions contains options for uploading a video to Bilibili.
type UploadOptions struct {
	VideoPath     string
	CoverPath     string
	Title         string
	Desc          string
	Source        string
	Tags          []string
	Tid           int
	Copyright     int // 1 = original, 2 = reupload
	UserCookiePath string
	CustomPath    string
	ShowProgress  bool
}

// Upload uploads a video to Bilibili using biliup.
func Upload(opts UploadOptions) error {
	path := "biliup"
	if opts.CustomPath != "" {
		path = opts.CustomPath
	}

	args := []string{"upload"}

	// Add cookie path if specified
	if opts.UserCookiePath != "" {
		args = append(args, "--user-cookie", opts.UserCookiePath)
	}

	// Add upload options
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	if opts.Desc != "" {
		args = append(args, "--desc", opts.Desc)
	}
	if opts.Source != "" {
		args = append(args, "--source", opts.Source)
	}
	if len(opts.Tags) > 0 {
		args = append(args, "--tag", strings.Join(opts.Tags, ","))
	}
	if opts.Tid > 0 {
		args = append(args, "--tid", fmt.Sprintf("%d", opts.Tid))
	}
	if opts.Copyright > 0 {
		args = append(args, "--copyright", fmt.Sprintf("%d", opts.Copyright))
	}
	if opts.CoverPath != "" {
		args = append(args, "--cover", opts.CoverPath)
	}

	// Add video path
	args = append(args, opts.VideoPath)

	cmd := exec.Command(path, args...)

	if opts.ShowProgress {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("biliup upload failed: %w", err)
	}

	return nil
}
