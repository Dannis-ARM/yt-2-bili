package subtitle

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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
	if got := chineseSubtitlePath(video); got != `C:\tmp\abc123.zh.srt` {
		t.Fatalf("unexpected Chinese subtitle path: %s", got)
	}
	if got := chineseSubtitledVideoPath(video); got != `C:\tmp\abc123.zh.subtitled.mp4` {
		t.Fatalf("unexpected Chinese subtitled video path: %s", got)
	}
}

func TestBuildWhisperArgsUsesOnlyExplicitOptions(t *testing.T) {
	args := buildWhisperArgs(Options{VideoPath: `C:\tmp\abc123.mp4`})
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

func TestEnsureSoftSubtitledReusesValidatedChineseSubtitle(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "abc123.mp4")
	writeTestFile(t, videoPath, "video")
	writeTestFile(t, subtitlePath(videoPath), "1\n00:00:00,000 --> 00:00:01,000\nHello\n")
	writeTestFile(t, chineseSubtitlePath(videoPath), "1\n00:00:00,000 --> 00:00:01,000\n你好\n")

	result, err := prepareSubtitleFiles(context.Background(), Options{
		VideoPath:              videoPath,
		SubtitleTargetLanguage: "zh",
		SubtitleMode:           ModeSoft,
		Translator:             failingTranslator{},
	})
	if err != nil {
		t.Fatalf("prepare subtitle files failed: %v", err)
	}
	if !result.ReusedChineseSubtitle {
		t.Fatal("expected existing Chinese subtitle to be reused")
	}
	if result.ChineseSubtitlePath != chineseSubtitlePath(videoPath) {
		t.Fatalf("unexpected Chinese subtitle path: %s", result.ChineseSubtitlePath)
	}
	if result.SubtitledVideoPath != chineseSubtitledVideoPath(videoPath) {
		t.Fatalf("unexpected Chinese subtitled video path: %s", result.SubtitledVideoPath)
	}
}

func TestPrepareSubtitleFilesTranslatesWhenNoChineseSRT(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "abc123.mp4")
	writeTestFile(t, videoPath, "video")
	sourceSRT := "1\n00:00:00,000 --> 00:00:01,000\nHello\n"
	writeTestFile(t, subtitlePath(videoPath), sourceSRT)

	mock := &mockTranslator{translate: func(ctx context.Context, srt string) (string, error) {
		if srt != sourceSRT {
			t.Fatalf("translator received unexpected source SRT:\n%s", srt)
		}
		return "1\n00:00:00,000 --> 00:00:01,000\n你好\n", nil
	}}

	result, err := prepareSubtitleFiles(context.Background(), Options{
		VideoPath:              videoPath,
		SubtitleTargetLanguage: "zh",
		Translator:             mock,
	})
	if err != nil {
		t.Fatalf("prepare subtitle files failed: %v", err)
	}
	if result.ChineseSubtitlePath != chineseSubtitlePath(videoPath) {
		t.Fatalf("unexpected Chinese subtitle path: %s", result.ChineseSubtitlePath)
	}
	if !mock.called {
		t.Fatal("expected translator to be called")
	}

	data, err := os.ReadFile(chineseSubtitlePath(videoPath))
	if err != nil {
		t.Fatalf("read Chinese subtitle file failed: %v", err)
	}
	if !strings.Contains(string(data), "你好") {
		t.Fatalf("expected Chinese subtitle file, got:\n%s", string(data))
	}
}

type mockTranslator struct {
	called       bool
	translate    func(ctx context.Context, srt string) (string, error)
	translateText func(ctx context.Context, text string) (string, error)
}

func (m *mockTranslator) TranslateSRT(ctx context.Context, srt string) (string, error) {
	m.called = true
	return m.translate(ctx, srt)
}

func (m *mockTranslator) TranslateText(ctx context.Context, text string) (string, error) {
	if m.translateText != nil {
		return m.translateText(ctx, text)
	}
	return text, nil
}

func TestBuildWhisperArgsAddsModelDirectory(t *testing.T) {
	args := buildWhisperArgs(Options{
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

func TestBuildWhisperArgsAddsDeviceAndComputeType(t *testing.T) {
	args := buildWhisperArgs(Options{
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

func TestTranslatorStreamsChineseSRT(t *testing.T) {
	server := newStreamingTranslationServer(t, "[1] 你好\n")
	defer server.Close()

	translator := NewLLMTranslator(LLMTranslatorOptions{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-v4-pro",
	})

	translated, err := translator.TranslateSRT(context.Background(), "1\n00:00:00,000 --> 00:00:01,000\nHello\n")
	if err != nil {
		t.Fatalf("translate failed: %v", err)
	}
	if !strings.Contains(translated, "你好") {
		t.Fatalf("expected Chinese subtitle text, got:\n%s", translated)
	}
}

func TestTranslatorFillsMissingEntriesWithOriginalText(t *testing.T) {
	// LLM returns fewer entries — missing ones use original text
	server := newStreamingTranslationServer(t, "[1] 你好\n")
	defer server.Close()

	translator := NewLLMTranslator(LLMTranslatorOptions{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-v4-pro",
	})

	// Source has 2 blocks but LLM returns 1 entry
	result, err := translator.TranslateSRT(context.Background(), "1\n00:00:00,000 --> 00:00:01,000\nHello\n\n2\n00:00:01,000 --> 00:00:02,000\nWorld\n")
	if err != nil {
		t.Fatalf("translate should not fail on missing entries: %v", err)
	}
	if !strings.Contains(result, "你好") {
		t.Fatal("expected first entry translated")
	}
	if !strings.Contains(result, "World") {
		t.Fatal("expected second entry to keep original text")
	}
}

func TestTranslatorSplitsBatchesWithoutSplittingBlocks(t *testing.T) {
	responses := []string{
		"[1] 你好\n",
		"[1] 世界\n",
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + strconv.Quote(responses[requests-1]) + "}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	translator := NewLLMTranslator(LLMTranslatorOptions{
		BaseURL:        server.URL,
		APIKey:         "test-key",
		Model:          "deepseek-v4-pro",
		BatchCharLimit: 5,
	})

	source := "1\n00:00:00,000 --> 00:00:01,000\nHello\n\n2\n00:00:01,000 --> 00:00:02,000\nWorld\n"
	translated, err := translator.TranslateSRT(context.Background(), source)
	if err != nil {
		t.Fatalf("translate failed: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 translation requests, got %d", requests)
	}
	if !strings.Contains(translated, "你好") || !strings.Contains(translated, "世界") {
		t.Fatalf("expected joined Chinese subtitle, got:\n%s", translated)
	}
}

func TestTranslatorRejectsInvalidStructureNoRetry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"1\\n00:00:02,000 --> 00:00:03,000\\n你好\\n\\n\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	translator := NewLLMTranslator(LLMTranslatorOptions{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "deepseek-v4-pro",
	})

	_, err := translator.TranslateSRT(context.Background(), "1\n00:00:00,000 --> 00:00:01,000\nHello\n")
	if err == nil {
		t.Fatal("translation with changed timeline should fail")
	}
	if requests != 1 {
		t.Fatalf("expected no retry, got %d requests", requests)
	}
}

func TestTranslatorDoesNotRetryAuthenticationErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	translator := NewLLMTranslator(LLMTranslatorOptions{
		BaseURL: server.URL,
		APIKey:  "bad-key",
		Model:   "deepseek-v4-pro",
	})

	_, err := translator.TranslateSRT(context.Background(), "1\n00:00:00,000 --> 00:00:01,000\nHello\n")
	if err == nil {
		t.Fatal("authentication error should fail")
	}
	if requests != 1 {
		t.Fatalf("authentication error should not retry, got %d requests", requests)
	}
}

func newStreamingTranslationServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + strconv.Quote(content) + "}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
}

type failingTranslator struct{}

func (failingTranslator) TranslateSRT(context.Context, string) (string, error) {
	return "", fmt.Errorf("translator should not be called")
}

func (failingTranslator) TranslateText(context.Context, string) (string, error) {
	return "", fmt.Errorf("translator should not be called")
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file failed: %v", err)
	}
}
