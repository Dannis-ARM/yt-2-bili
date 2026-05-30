// Package whisperjson provides parsing for Whisper JSON output format with word-level timestamps.
package whisperjson

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// WhisperOutput represents the top-level structure of Whisper JSON output.
type WhisperOutput struct {
	Segments []Segment `json:"segments"`
}

// Segment represents a single segment in Whisper output.
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Words []Word  `json:"words"`
}

// Word represents a single word with timestamps.
type Word struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Probability float64 `json:"probability"`
}

// StartTime returns the start time as time.Duration.
func (w Word) StartTime() time.Duration {
	return time.Duration(w.Start * float64(time.Second))
}

// EndTime returns the end time as time.Duration.
func (w Word) EndTime() time.Duration {
	return time.Duration(w.End * float64(time.Second))
}

// Parse parses Whisper JSON content.
func Parse(content []byte) (WhisperOutput, error) {
	var output WhisperOutput
	if err := json.Unmarshal(content, &output); err != nil {
		return WhisperOutput{}, fmt.Errorf("whisperjson parse: %w", err)
	}
	return output, nil
}

// ParseFile reads and parses a Whisper JSON file.
func ParseFile(path string) (WhisperOutput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WhisperOutput{}, fmt.Errorf("whisperjson read file: %w", err)
	}
	return Parse(data)
}

// FlattenWords returns all words from all segments as a single slice.
func (w WhisperOutput) FlattenWords() []Word {
	var result []Word
	for _, seg := range w.Segments {
		result = append(result, seg.Words...)
	}
	return result
}
