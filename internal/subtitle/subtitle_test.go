package subtitle

import (
	"reflect"
	"testing"
)

func TestSubtitleOutputPaths(t *testing.T) {
	video := `C:\tmp\abc123.mp4`

	if got := subtitlePath(video); got != `C:\tmp\abc123.srt` {
		t.Fatalf("unexpected subtitle path: %s", got)
	}
	if got := subtitledVideoPath(video); got != `C:\tmp\abc123.subtitled.mp4` {
		t.Fatalf("unexpected subtitled video path: %s", got)
	}
}

func TestBuildWhisperArgsUsesOnlyExplicitOptions(t *testing.T) {
	args := buildWhisperArgs(Options{VideoPath: `C:\tmp\abc123.mp4`})
	expected := []string{`C:\tmp\abc123.mp4`, "--output_format", "srt", "--output_dir", `C:\tmp`}

	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildWhisperArgsAddsOverrides(t *testing.T) {
	args := buildWhisperArgs(Options{
		VideoPath: `C:\tmp\abc123.mp4`,
		Model:     "small",
		Device:    "cuda",
		Language:  "en",
		Threads:   4,
	})
	expected := []string{
		`C:\tmp\abc123.mp4`, "--output_format", "srt", "--output_dir", `C:\tmp`,
		"--model", "small",
		"--device", "cuda",
		"--language", "en",
		"--threads", "4",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}
