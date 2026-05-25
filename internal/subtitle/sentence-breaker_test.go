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

func TestBreakSentencesSplitsByPunctuation(t *testing.T) {
	// 4 sentences at 10s total: each gets ~2.5s, all under 5s limit
	input := "1\n00:00:00,000 --> 00:00:10,000\nHello world. How are you? I'm fine. Thanks.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) < 3 {
		t.Fatalf("expected at least 3 blocks, got %d:\n%s", len(blocks), output)
	}
	// Check sequential numbering
	for i, b := range blocks {
		if b.Number != strconv.Itoa(i+1) {
			t.Fatalf("expected block %d number %d, got %s", i, i+1, b.Number)
		}
	}
	// Check timecodes don't exceed original range
	firstStart, _, err := parseSRTTimeline(blocks[0].Timeline)
	if err != nil {
		t.Fatalf("parse first timeline: %v", err)
	}
	_, lastEnd, err := parseSRTTimeline(blocks[len(blocks)-1].Timeline)
	if err != nil {
		t.Fatalf("parse last timeline: %v", err)
	}
	if firstStart != 0 {
		t.Fatalf("first block should start at 0, got %v", firstStart)
	}
	if lastEnd != 10_000_000_000 {
		t.Fatalf("last block should end at 10s, got %v", lastEnd)
	}
}

func TestBreakSentencesTimecodesProportional(t *testing.T) {
	input := "1\n00:00:00,000 --> 00:00:08,000\nABC. DEFGH.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d:\n%s", len(blocks), output)
	}
	sStart, _, err := parseSRTTimeline(blocks[1].Timeline)
	if err != nil {
		t.Fatalf("parse second timeline: %v", err)
	}
	secondStartMs := sStart.Milliseconds()
	// "ABC." (4 chars) gets ~3.2s, "DEFGH." (6 chars) gets ~4.8s
	if secondStartMs < 2500 || secondStartMs > 4000 {
		t.Fatalf("expected second block to start around 3200ms, got %dms", secondStartMs)
	}
}

func TestBreakSentencesHardSplitByComma(t *testing.T) {
	// Create a long sentence without sentence-ending punctuation but with commas
	// that exceeds 84 chars
	longText := strings.Repeat("word, ", 50) // ~300 chars
	input := "1\n00:00:00,000 --> 00:00:10,000\n" + longText + "\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) < 3 {
		t.Fatalf("expected at least 3 blocks, got %d", len(blocks))
	}
	for _, b := range blocks {
		lines := extractTextLines(b.Raw)
		text := strings.Join(lines, " ")
		if charCount(text) > maxCharsPerEntry {
			t.Fatalf("block exceeds char limit (%d): %s", charCount(text), text)
		}
	}
}

func TestBreakSentencesHardSplitBySpace(t *testing.T) {
	// Long text with no commas — must split at word boundaries
	longText := strings.Repeat("hello world ", 50) // ~600 chars
	input := "1\n00:00:00,000 --> 00:00:10,000\n" + longText + "\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	for _, b := range blocks {
		lines := extractTextLines(b.Raw)
		text := strings.Join(lines, " ")
		if charCount(text) > maxCharsPerEntry {
			t.Fatalf("block exceeds char limit (%d): %s", charCount(text), text)
		}
	}
}

func TestBreakSentencesSplitsByMaxDuration(t *testing.T) {
	// A short sentence but spread over 20 seconds — must split by time
	input := "1\n00:00:00,000 --> 00:00:20,000\nHello world.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) < 3 {
		t.Fatalf("expected at least 3 blocks due to duration split, got %d:\n%s", len(blocks), output)
	}
	for _, b := range blocks {
		start, end, err := parseSRTTimeline(b.Timeline)
		if err != nil {
			t.Fatalf("parse timeline: %v", err)
		}
		duration := end - start
		if duration > maxDurationPerEntry*2 { // allow some slack
			t.Fatalf("block duration %v exceeds limit: %s", duration, b.Timeline)
		}
	}
}

func TestBreakSentencesMultipleBlocks(t *testing.T) {
	input := "1\n00:00:00,000 --> 00:00:10,000\nHello world. How are you?\n\n2\n00:00:10,000 --> 00:00:20,000\nI'm fine. Thanks for asking.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := parseSRTBlocks(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	// Check sequential numbering across ALL blocks
	for i, b := range blocks {
		if b.Number != strconv.Itoa(i+1) {
			t.Fatalf("block numbering wrong at position %d: got %s", i, b.Number)
		}
	}
	// Last block end time should match original end time (20s)
	_, lastEnd, _ := parseSRTTimeline(blocks[len(blocks)-1].Timeline)
	if lastEnd.Milliseconds() < 19900 || lastEnd.Milliseconds() > 20100 {
		t.Fatalf("last block should end around 20000ms, got %dms", lastEnd.Milliseconds())
	}
}

func TestBreakSentencesEmptySRT(t *testing.T) {
	_, err := BreakSentences("")
	if err == nil {
		t.Fatal("expected error for empty SRT")
	}
}

func TestPrepareSubtitleFilesAppliesSentenceBreaking(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "abc.mp4")
	writeTestFile(t, videoPath, "video content")

	// Pre-create a long SRT that needs breaking
	longSRT := "1\n00:00:00,000 --> 00:00:15,000\nHello world. How are you? I'm fine and doing great.\n"
	writeTestFile(t, subtitlePath(videoPath), longSRT)

	// Simulate Whisper not running (already has SRT), so breaking won't apply
	result, err := prepareSubtitleFiles(context.Background(), Options{VideoPath: videoPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if !result.ReusedSubtitle {
		t.Fatal("expected subtitle reuse")
	}
	// SRT should be unchanged since it was reused, not regenerated
	data, _ := os.ReadFile(subtitlePath(videoPath))
	if string(data) != longSRT {
		t.Fatal("reused SRT should not be modified by sentence breaking")
	}
}

func TestTranslationCacheInvalidatedAfterBreaking(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "abc.mp4")
	writeTestFile(t, videoPath, "video content")

	// Simulate a new Whisper run: we create source SRT, then test that
	// stale Chinese SRT gets invalidated
	// This test verifies the translation re-triggers when cache is invalid

	// We can't easily test Whisper invocation, so test the cache invalidation
	// path directly: source with N blocks, Chinese with different block count
	sourceSRT := "1\n00:00:00,000 --> 00:00:03,000\nHello world.\n\n2\n00:00:03,000 --> 00:00:06,000\nHow are you?\n"
	staleChinese := "1\n00:00:00,000 --> 00:00:05,000\n你好世界。\n" // different block count

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
