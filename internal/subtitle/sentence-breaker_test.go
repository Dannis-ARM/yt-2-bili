package subtitle

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBreakSentencesShortEntryUnchanged(t *testing.T) {
	input := "1\n00:00:00,000 --> 00:00:03,000\nHello.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Number != "1" {
		t.Fatalf("expected number 1, got %s", blocks[0].Number)
	}
	if blocks[0].Timeline != "00:00:00,000 --> 00:00:03,000" {
		t.Fatalf("timeline changed: %s", blocks[0].Timeline)
	}
}

func TestBreakSentencesSplitsLongCharEntry(t *testing.T) {
	longText := strings.Repeat("a", maxCharsPerEntry*2+10)
	input := "1\n00:00:00,000 --> 00:00:03,000\n" + longText + "\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(blocks))
	}
	for _, b := range blocks {
		text := strings.Join(extractTextLines(b.Text), " ")
		if charCount(text) > maxCharsPerEntry {
			t.Fatalf("block exceeds char limit (%d): %s", charCount(text), text)
		}
	}
}

func TestBreakSentencesSplitsLongDurationEntry(t *testing.T) {
	input := "1\n00:00:00,000 --> 00:00:20,000\nHello world.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks due to duration, got %d", len(blocks))
	}
}

func TestBreakSentencesMultipleBlocks(t *testing.T) {
	input := "1\n00:00:00,000 --> 00:00:03,000\nHello.\n\n2\n00:00:03,000 --> 00:00:06,000\nWorld.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	for i, b := range blocks {
		if b.Number != strconv.Itoa(i+1) {
			t.Fatalf("block numbering wrong at position %d: got %s", i, b.Number)
		}
	}
}

func TestBreakSentencesEmptySRT(t *testing.T) {
	result, err := BreakSentences("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "\n" {
		t.Fatalf("expected empty result with trailing newline, got: %q", result)
	}
}

func TestPrepareSubtitleFilesAppliesSentenceBreaking(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "abc.mp4")
	writeTestFile(t, videoPath, "video content")

	longSRT := "1\n00:00:00,000 --> 00:00:15,000\nHello world.\n"
	writeTestFile(t, subtitlePath(videoPath), longSRT)

	result, err := prepareSubtitleFiles(context.Background(), Options{VideoPath: videoPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if !result.ReusedSubtitle {
		t.Fatal("expected subtitle reuse")
	}
	data, _ := os.ReadFile(subtitlePath(videoPath))
	if string(data) != longSRT {
		t.Fatal("reused SRT should not be modified by sentence breaking")
	}
}

func TestTranslationCacheInvalidatedAfterBreaking(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "abc.mp4")
	writeTestFile(t, videoPath, "video content")

	sourceSRT := "1\n00:00:00,000 --> 00:00:03,000\nHello world.\n\n2\n00:00:03,000 --> 00:00:06,000\nHow are you?\n"
	staleChinese := "1\n00:00:00,000 --> 00:00:05,000\n你好世界。\n"

	writeTestFile(t, subtitlePath(videoPath), sourceSRT)
	writeTestFile(t, chineseSubtitlePath(videoPath), staleChinese)

	mock := &mockTranslator{translate: func(ctx context.Context, srt string) (string, error) {
		return "1\n00:00:00,000 --> 00:00:03,000\n你好世界。\n\n2\n00:00:03,000 --> 00:00:06,000\n你好吗？\n", nil
	}}

	result, err := prepareSubtitleFiles(context.Background(), Options{
		VideoPath:              videoPath,
		SubtitleTargetLanguage: "zh",
		Translator:             mock,
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if !mock.called {
		t.Fatal("translator should be called because stale cache was invalidated")
	}
	if result.ReusedChineseSubtitle {
		t.Fatal("should not reuse stale Chinese subtitle")
	}
}
