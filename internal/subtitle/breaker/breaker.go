package breaker

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dannis/yt-2-bili/internal/subtitle/srt"
)

const (
	maxCharsPerEntry    = 84
	maxDurationPerEntry = 5 * time.Second
)

type timedSegment struct {
	text  string
	start time.Duration
	end   time.Duration
}

// BreakSentences is a safety net that only splits blocks exceeding maxCharsPerEntry or maxDurationPerEntry.
// It trusts that whisper-ctranslate2 has already done proper sentence segmentation via its configuration.
func BreakSentences(srtContent string) (string, error) {
	blocks, err := srt.Parse(srtContent)
	if err != nil {
		return "", fmt.Errorf("sentence breaking: %w", err)
	}

	var result []srt.Block
	for _, block := range blocks {
		splitBlocks := splitBlockIfNeeded(block)
		result = append(result, splitBlocks...)
	}

	for i := range result {
		result[i].Number = strconv.Itoa(i + 1)
		result[i].Text = cleanText(result[i].Text)
	}

	return srt.Format(result), nil
}

// ApplyToFile applies sentence breaking to an SRT file in-place.
func ApplyToFile(srtPath string) error {
	data, err := os.ReadFile(srtPath)
	if err != nil {
		return fmt.Errorf("sentence breaking: read source srt: %w", err)
	}
	broken, err := BreakSentences(string(data))
	if err != nil {
		return fmt.Errorf("sentence breaking: %w", err)
	}
	if err := os.WriteFile(srtPath, []byte(broken), 0o644); err != nil {
		return fmt.Errorf("sentence breaking: write broken srt: %w", err)
	}
	return nil
}

func splitBlockIfNeeded(block srt.Block) []srt.Block {
	text := strings.Join(extractTextLines(block.Text), " ")
	duration := block.End - block.Start
	chars := charCount(text)

	if duration <= maxDurationPerEntry && chars <= maxCharsPerEntry {
		return []srt.Block{block}
	}

	numParts := 1
	if chars > maxCharsPerEntry {
		numParts = (chars + maxCharsPerEntry - 1) / maxCharsPerEntry
	}
	if duration > maxDurationPerEntry {
		durationParts := int(math.Ceil(float64(duration) / float64(maxDurationPerEntry)))
		if durationParts > numParts {
			numParts = durationParts
		}
	}

	if numParts <= 1 {
		return []srt.Block{block}
	}

	return splitBlockEvenly(text, block.Start, block.End, numParts)
}

func splitBlockEvenly(text string, start, end time.Duration, numParts int) []srt.Block {
	runes := []rune(text)
	totalChars := len(runes)
	totalDuration := end - start
	charsPerPart := (totalChars + numParts - 1) / numParts
	durationPerPart := totalDuration / time.Duration(numParts)

	var result []timedSegment
	current := start
	charPos := 0

	for i := 0; i < numParts; i++ {
		if charPos >= totalChars {
			break
		}

		partChars := charsPerPart
		if charPos+partChars > totalChars {
			partChars = totalChars - charPos
		}

		partText := string(runes[charPos : charPos+partChars])
		partText = strings.TrimSpace(partText)
		if partText == "" {
			charPos += partChars
			continue
		}

		partEnd := current + durationPerPart
		if i == numParts-1 {
			partEnd = end
		}

		result = append(result, timedSegment{
			text:  partText,
			start: current,
			end:   partEnd,
		})

		charPos += partChars
		current = partEnd
	}

	blocks := make([]srt.Block, len(result))
	for i, seg := range result {
		blocks[i] = srt.Block{
			Start: seg.start,
			End:   seg.end,
			Text:  seg.text,
		}
	}
	return blocks
}

func charCount(s string) int {
	return len([]rune(s))
}

func extractTextLines(text string) []string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		return []string{text}
	}
	return result
}

func cleanText(text string) string {
	return strings.Join(extractTextLines(text), "\n")
}
