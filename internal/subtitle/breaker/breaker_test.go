package breaker

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dannis/yt-2-bili/internal/subtitle/srt"
)

func TestBreakSentencesShortEntryUnchanged(t *testing.T) {
	input := "1\n00:00:00,000 --> 00:00:03,000\nHello.\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := srt.Parse(output)
	if err != nil {
		t.Fatalf("output parse failed: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Number != "1" {
		t.Fatalf("expected number 1, got %s", blocks[0].Number)
	}
	if blocks[0].Start != 0 {
		t.Fatalf("unexpected start: %v", blocks[0].Start)
	}
	if blocks[0].End != 3*time.Second {
		t.Fatalf("unexpected end: %v", blocks[0].End)
	}
}

func TestBreakSentencesSplitsLongCharEntry(t *testing.T) {
	longText := strings.Repeat("a", maxCharsPerEntry*2+10)
	input := "1\n00:00:00,000 --> 00:00:03,000\n" + longText + "\n"
	output, err := BreakSentences(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks, err := srt.Parse(output)
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
	blocks, err := srt.Parse(output)
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
	blocks, err := srt.Parse(output)
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

func TestApplyToFile(t *testing.T) {
	dir := t.TempDir()
	srtPath := filepath.Join(dir, "test.srt")
	longSRT := "1\n00:00:00,000 --> 00:00:15,000\nHello world.\n"
	os.WriteFile(srtPath, []byte(longSRT), 0o644)

	err := ApplyToFile(srtPath)
	if err != nil {
		t.Fatalf("ApplyToFile failed: %v", err)
	}

	data, err := os.ReadFile(srtPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) == longSRT {
		t.Fatal("SRT should have been modified by sentence breaking")
	}
}
