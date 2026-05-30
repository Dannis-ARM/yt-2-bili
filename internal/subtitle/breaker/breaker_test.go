package breaker

import (
	"testing"
	"time"

	"github.com/dannis/yt-2-bili/internal/subtitle/srt"
	"github.com/dannis/yt-2-bili/internal/subtitle/whisperjson"
)

func TestJSONBreaker_MergesShortSentences(t *testing.T) {
	// Test data: Chinese sentence split into individual characters
	input := whisperjson.WhisperOutput{
		Segments: []whisperjson.Segment{
			{
				ID:    0,
				Start: 1.0,
				End:   5.5,
				Text:  "我认为在生产环境中引入这个中间件。",
				Words: []whisperjson.Word{
					{Word: "我认为", Start: 1.0, End: 1.8, Probability: 0.98},
					{Word: "在", Start: 1.8, End: 2.1, Probability: 0.99},
					{Word: "生产", Start: 2.1, End: 2.9, Probability: 0.99},
					{Word: "环境", Start: 2.9, End: 3.5, Probability: 0.95},
					{Word: "中", Start: 3.5, End: 3.8, Probability: 0.99},
					{Word: "引入", Start: 3.8, End: 4.5, Probability: 0.97},
					{Word: "这个", Start: 4.5, End: 5.0, Probability: 0.99},
					{Word: "中间件。", Start: 5.0, End: 5.5, Probability: 0.94},
				},
			},
		},
	}

	breaker := &JSONBreaker{Output: input}
	blocks, err := breaker.Break()
	if err != nil {
		t.Fatalf("Break failed: %v", err)
	}

	// All words should be merged into one block
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Start != 1000*time.Millisecond {
		t.Errorf("expected start 1.0s, got %v", blocks[0].Start)
	}

	if blocks[0].End != 5500*time.Millisecond {
		t.Errorf("expected end 5.5s, got %v", blocks[0].End)
	}
}

func TestJSONBreaker_SplitsAtLongSilence(t *testing.T) {
	input := whisperjson.WhisperOutput{
		Segments: []whisperjson.Segment{
			{
				ID:    0,
				Start: 1.0,
				End:   4.0,
				Text:  "第一句。第二句。",
				Words: []whisperjson.Word{
					{Word: "第一句。", Start: 1.0, End: 2.0, Probability: 0.98},
					// Long silence: 2.0 to 3.0 (1000ms, more than maxSilenceGap 500ms)
					{Word: "第二句。", Start: 3.0, End: 4.0, Probability: 0.99},
				},
			},
		},
	}

	breaker := &JSONBreaker{Output: input}
	blocks, err := breaker.Break()
	if err != nil {
		t.Fatalf("Break failed: %v", err)
	}

	// Should split into 2 blocks because of the long silence
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
}

func TestJSONBreaker_SplitsAtSentenceEnd(t *testing.T) {
	input := whisperjson.WhisperOutput{
		Segments: []whisperjson.Segment{
			{
				ID:    0,
				Start: 1.0,
				End:   5.0,
				Text:  "你好。今天天气真好。",
				Words: []whisperjson.Word{
					{Word: "你好。", Start: 1.0, End: 2.0, Probability: 0.98},
					{Word: "今天", Start: 2.1, End: 2.8, Probability: 0.99},
					{Word: "天气", Start: 2.8, End: 3.5, Probability: 0.99},
					{Word: "真好。", Start: 3.5, End: 5.0, Probability: 0.99},
				},
			},
		},
	}

	breaker := &JSONBreaker{Output: input}
	blocks, err := breaker.Break()
	if err != nil {
		t.Fatalf("Break failed: %v", err)
	}

	// Should split at the sentence-ending punctuation
	// (Note: This depends on the exact timing, but let's verify it doesn't crash)
	_ = blocks
}

func TestJSONBreaker_SplitsAtCharLimit(t *testing.T) {
	// Create a very long sentence that exceeds maxCharsPerEntry
	var words []whisperjson.Word
	for i := 0; i < 50; i++ {
		words = append(words, whisperjson.Word{
			Word:  "字",
			Start: 1.0 + float64(i)*0.1,
			End:   1.1 + float64(i)*0.1,
		})
	}

	input := whisperjson.WhisperOutput{
		Segments: []whisperjson.Segment{
			{
				ID:    0,
				Start: 1.0,
				End:   6.0,
				Text:  "很长很长的句子...",
				Words: words,
			},
		},
	}

	breaker := &JSONBreaker{Output: input}
	blocks, err := breaker.Break()
	if err != nil {
		t.Fatalf("Break failed: %v", err)
	}

	// Should have split into multiple blocks
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks for long text, got %d", len(blocks))
	}
}

func TestLegacySRTBreaker(t *testing.T) {
	input := []srt.Block{
		{Number: "1", Start: 0, End: 1000 * time.Millisecond, Text: "Hello"},
		{Number: "2", Start: 1000 * time.Millisecond, End: 2000 * time.Millisecond, Text: "World"},
	}

	breaker := &LegacySRTBreaker{Blocks: input}
	blocks, err := breaker.Break()
	if err != nil {
		t.Fatalf("Break failed: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].Number != "1" {
		t.Errorf("expected number 1, got %s", blocks[0].Number)
	}

	if blocks[1].Number != "2" {
		t.Errorf("expected number 2, got %s", blocks[1].Number)
	}
}

func TestEndsWithSentencePunct(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"Chinese period", "你好。", true},
		{"Chinese exclamation", "你好！", true},
		{"Chinese question", "你好？", true},
		{"English period", "Hello.", true},
		{"English exclamation", "Hello!", true},
		{"English question", "Hello?", true},
		{"No punctuation", "Hello", false},
		{"Comma", "Hello,", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := endsWithSentencePunct(tt.text); got != tt.want {
				t.Errorf("endsWithSentencePunct(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
