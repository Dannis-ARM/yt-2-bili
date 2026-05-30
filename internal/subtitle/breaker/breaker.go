// Package breaker provides intelligent subtitle merging and splitting.
package breaker

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dannis/yt-2-bili/internal/subtitle/srt"
	"github.com/dannis/yt-2-bili/internal/subtitle/whisperjson"
)

const (
	// maxCharsPerEntry is the maximum number of characters per subtitle block.
	// For Chinese, 40 characters is a comfortable reading length (about 2 lines).
	maxCharsPerEntry = 40

	// maxDurationPerEntry is the maximum duration per subtitle block.
	maxDurationPerEntry = 5 * time.Second

	// maxSilenceGap is the maximum silence between words that we'll allow merging.
	// If there's a gap longer than this, we'll start a new subtitle block.
	maxSilenceGap = 500 * time.Millisecond
)

// SubtitleBreaker is the interface for subtitle processing.
type SubtitleBreaker interface {
	Break() ([]srt.Block, error)
}

// JSONBreaker implements high-precision word-level breaking from Whisper JSON.
type JSONBreaker struct {
	Output whisperjson.WhisperOutput
}

// LegacySRTBreaker implements backward compatibility for existing SRT files.
type LegacySRTBreaker struct {
	Blocks []srt.Block
}

// Break processes Whisper JSON with word-level timestamps and returns optimized SRT blocks.
func (j *JSONBreaker) Break() ([]srt.Block, error) {
	words := j.Output.FlattenWords()
	if len(words) == 0 {
		return nil, nil
	}

	var result []srt.Block
	var currentWords []whisperjson.Word
	var currentText string

	for i, word := range words {
		// Check if we should start a new block
		shouldStartNewBlock := false

		if len(currentWords) > 0 {
			// Check 1: Would adding this word exceed char limit?
			proposedText := currentText + cleanWord(word.Word)
			if utf8.RuneCountInString(proposedText) > maxCharsPerEntry {
				shouldStartNewBlock = true
			}

			// Check 2: Would adding this word exceed duration limit?
			if !shouldStartNewBlock {
				blockStart := currentWords[0].StartTime()
				blockEnd := word.EndTime()
				if blockEnd-blockStart > maxDurationPerEntry {
					shouldStartNewBlock = true
				}
			}

			// Check 3: Is there a long silence after the previous word?
			if !shouldStartNewBlock {
				prevWord := currentWords[len(currentWords)-1]
				silenceGap := word.StartTime() - prevWord.EndTime()
				if silenceGap > maxSilenceGap {
					shouldStartNewBlock = true
				}
			}

			// Check 4: Did the previous word end with sentence-ending punctuation?
			if !shouldStartNewBlock {
				prevText := cleanWord(currentWords[len(currentWords)-1].Word)
				if endsWithSentencePunct(prevText) {
					shouldStartNewBlock = true
				}
			}
		}

		if shouldStartNewBlock && len(currentWords) > 0 {
			// Seal the current block
			block := buildBlock(currentWords, currentText, len(result)+1)
			result = append(result, block)

			// Reset for next block
			currentWords = nil
			currentText = ""
		}

		// Add the current word
		currentWords = append(currentWords, word)
		currentText = currentText + cleanWord(word.Word)

		// If this is the last word, seal the block
		if i == len(words)-1 {
			block := buildBlock(currentWords, currentText, len(result)+1)
			result = append(result, block)
		}
	}

	return result, nil
}

// Break processes legacy SRT files with linear interpolation fallback.
func (l *LegacySRTBreaker) Break() ([]srt.Block, error) {
	// For now, we just return the blocks as-is with re-numbering.
	// In the future, we could implement merging/splitting logic here.
	result := make([]srt.Block, len(l.Blocks))
	for i, b := range l.Blocks {
		result[i] = srt.Block{
			Number: fmt.Sprintf("%d", i+1),
			Start:  b.Start,
			End:    b.End,
			Text:   cleanText(b.Text),
		}
	}
	return result, nil
}

// BreakSentences is the backward-compatible entrypoint from SRT.
func BreakSentences(srtContent string) (string, error) {
	blocks, err := srt.Parse(srtContent)
	if err != nil {
		return "", fmt.Errorf("breaker: %w", err)
	}

	breaker := &LegacySRTBreaker{Blocks: blocks}
	result, err := breaker.Break()
	if err != nil {
		return "", err
	}

	return srt.Format(result), nil
}

// ApplyToFile applies sentence breaking to an SRT file in-place (legacy).
func ApplyToFile(srtPath string) error {
	data, err := os.ReadFile(srtPath)
	if err != nil {
		return fmt.Errorf("breaker: read source srt: %w", err)
	}
	broken, err := BreakSentences(string(data))
	if err != nil {
		return fmt.Errorf("breaker: %w", err)
	}
	if err := os.WriteFile(srtPath, []byte(broken), 0o644); err != nil {
		return fmt.Errorf("breaker: write broken srt: %w", err)
	}
	return nil
}

// ProcessJSONFile reads a Whisper JSON file and writes an optimized SRT file.
func ProcessJSONFile(jsonPath, srtPath string) error {
	output, err := whisperjson.ParseFile(jsonPath)
	if err != nil {
		return fmt.Errorf("breaker: %w", err)
	}

	breaker := &JSONBreaker{Output: output}
	blocks, err := breaker.Break()
	if err != nil {
		return err
	}

	return srt.Write(srtPath, blocks)
}

// ProcessJSON processes Whisper JSON data and returns SRT content.
func ProcessJSON(jsonData []byte) (string, error) {
	output, err := whisperjson.Parse(jsonData)
	if err != nil {
		return "", fmt.Errorf("breaker: %w", err)
	}

	breaker := &JSONBreaker{Output: output}
	blocks, err := breaker.Break()
	if err != nil {
		return "", err
	}

	return srt.Format(blocks), nil
}

// Helper functions

func buildBlock(words []whisperjson.Word, text string, number int) srt.Block {
	return srt.Block{
		Number: fmt.Sprintf("%d", number),
		Start:  words[0].StartTime(),
		End:    words[len(words)-1].EndTime(),
		Text:   cleanText(text),
	}
}

func cleanWord(word string) string {
	// Remove leading/trailing whitespace
	word = strings.TrimSpace(word)
	// For Chinese, we don't need spaces between words.
	// Just return as-is - Whisper usually doesn't add spaces for Chinese anyway.
	return word
}

func cleanText(text string) string {
	// Normalize whitespace
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	return text
}

func endsWithSentencePunct(s string) bool {
	if len(s) == 0 {
		return false
	}
	lastRune, _ := utf8.DecodeLastRuneInString(s)
	switch lastRune {
	case '。', '！', '？', '.', '!', '?':
		return true
	default:
		return false
	}
}
