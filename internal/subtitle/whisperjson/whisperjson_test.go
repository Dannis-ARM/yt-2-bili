package whisperjson

import (
	"testing"
	"time"
)

func TestParseSimple(t *testing.T) {
	input := `{
		"segments": [
			{
				"id": 0,
				"start": 1.0,
				"end": 3.0,
				"text": "Hello world",
				"words": [
					{"word": "Hello", "start": 1.0, "end": 1.8, "probability": 0.98},
					{"word": "world", "start": 1.8, "end": 3.0, "probability": 0.99}
				]
			}
		]
	}`

	output, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(output.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(output.Segments))
	}

	if len(output.Segments[0].Words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(output.Segments[0].Words))
	}

	if output.Segments[0].Words[0].Word != "Hello" {
		t.Fatalf("expected first word to be Hello, got %s", output.Segments[0].Words[0].Word)
	}
}

func TestWordTimeConversion(t *testing.T) {
	w := Word{Start: 1.5, End: 2.25}

	if w.StartTime() != 1500*time.Millisecond {
		t.Fatalf("expected start time 1.5s, got %v", w.StartTime())
	}

	if w.EndTime() != 2250*time.Millisecond {
		t.Fatalf("expected end time 2.25s, got %v", w.EndTime())
	}
}

func TestFlattenWords(t *testing.T) {
	input := `{
		"segments": [
			{
				"id": 0,
				"start": 1.0,
				"end": 3.0,
				"text": "First part",
				"words": [
					{"word": "First", "start": 1.0, "end": 1.8, "probability": 0.98},
					{"word": "part", "start": 1.8, "end": 3.0, "probability": 0.99}
				]
			},
			{
				"id": 1,
				"start": 3.5,
				"end": 5.0,
				"text": "Second part",
				"words": [
					{"word": "Second", "start": 3.5, "end": 4.2, "probability": 0.97},
					{"word": "part", "start": 4.2, "end": 5.0, "probability": 0.98}
				]
			}
		]
	}`

	output, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	words := output.FlattenWords()
	if len(words) != 4 {
		t.Fatalf("expected 4 flattened words, got %d", len(words))
	}
}
