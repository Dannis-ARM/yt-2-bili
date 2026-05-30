package whisper

import (
	"reflect"
	"testing"
)

func TestBuildArgsDefaultOptions(t *testing.T) {
	args := buildArgs(Options{VideoPath: `C:\tmp\abc123.mp4`})
	expected := []string{
		`C:\tmp\abc123.mp4`, "--output_format", "json", "--output_dir", `C:\tmp`,
		"--word_timestamps", "True",
		"--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--compute_type", "int8",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildArgsModelDirectory(t *testing.T) {
	args := buildArgs(Options{
		VideoPath:      `C:\tmp\abc123.mp4`,
		ModelDirectory: `E:\Models\faster-whisper-large-v3`,
	})
	expected := []string{
		`C:\tmp\abc123.mp4`, "--output_format", "json", "--output_dir", `C:\tmp`,
		"--word_timestamps", "True",
		"--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--compute_type", "int8",
		"--model_directory", `E:\Models\faster-whisper-large-v3`,
	}

	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildArgsDeviceAndComputeType(t *testing.T) {
	args := buildArgs(Options{
		VideoPath:          `C:\tmp\abc123.mp4`,
		WhisperDevice:      "cuda",
		WhisperComputeType: "float16",
	})
	expected := []string{
		`C:\tmp\abc123.mp4`, "--output_format", "json", "--output_dir", `C:\tmp`,
		"--word_timestamps", "True",
		"--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--compute_type", "float16",
		"--device", "cuda",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestJSONOutputPath(t *testing.T) {
	tests := []struct {
		name     string
		videoPath string
		want      string
	}{
		{"simple", "video.mp4", "video.json"},
		{"with spaces", "my video.mp4", "my video.json"},
		{"different ext", "video.mkv", "video.json"},
		{"with directory", "/tmp/video.mp4", "/tmp/video.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonOutputPath(tt.videoPath); got != tt.want {
				t.Errorf("jsonOutputPath(%q) = %q, want %q", tt.videoPath, got, tt.want)
			}
		})
	}
}
