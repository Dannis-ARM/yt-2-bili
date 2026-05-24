package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dannis/yt-2-bili/internal/workflow"
)

func TestHelpShowsConcreteDefaultOutputDir(t *testing.T) {
	help := executeHelp(t, "--help")

	if !strings.Contains(help, workflow.DefaultOutputDir()) {
		t.Fatalf("help should include default output directory %q, got:\n%s", workflow.DefaultOutputDir(), help)
	}
}

func TestHelpShowsWorkflowSubcommands(t *testing.T) {
	help := executeHelp(t, "--help")

	for _, command := range []string{"download", "upload", "transfer"} {
		if !strings.Contains(help, command) {
			t.Fatalf("help should include %q subcommand, got:\n%s", command, help)
		}
	}
}

func TestSubtitleTargetLanguageRequiresGenerateSubtitles(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"download", "--subtitle-target-language", "zh", "https://www.youtube.com/watch?v=test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("--subtitle-target-language without --generate-subtitles should fail")
	}
	if !strings.Contains(err.Error(), "requires --generate-subtitles") {
		t.Fatalf("expected missing generate-subtitles error, got: %v", err)
	}
}

func TestUploadRequiresTitle(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upload", "video.mp4"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("upload without title should fail")
	}
	if !strings.Contains(err.Error(), "--title is required") {
		t.Fatalf("expected missing title error, got: %v", err)
	}
}

func executeHelp(t *testing.T, args ...string) string {
	t.Helper()

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	return out.String()
}
