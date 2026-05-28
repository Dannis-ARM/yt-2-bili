package whisper

import (
	"reflect"
	"testing"
)

func TestBuildArgsDefaultOptions(t *testing.T) {
	args := buildArgs(Options{VideoPath: `C:\tmp\abc123.mp4`})
	expected := []string{
		`C:\tmp\abc123.mp4`, "--output_format", "srt", "--output_dir", `C:\tmp`,
		"--batched", "True", "--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--word_timestamps", "True",
		"--max_line_width", "42",
		"--max_line_count", "1",
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
		`C:\tmp\abc123.mp4`, "--output_format", "srt", "--output_dir", `C:\tmp`,
		"--batched", "True", "--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--word_timestamps", "True",
		"--max_line_width", "42",
		"--max_line_count", "1",
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
		`C:\tmp\abc123.mp4`, "--output_format", "srt", "--output_dir", `C:\tmp`,
		"--batched", "True", "--vad_filter", "True",
		"--vad_min_silence_duration_ms", "400",
		"--word_timestamps", "True",
		"--max_line_width", "42",
		"--max_line_count", "1",
		"--compute_type", "float16",
		"--device", "cuda",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}
