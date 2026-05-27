package subtitle

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
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
func BreakSentences(srt string) (string, error) {
	blocks, err := parseSRTBlocks(srt)
	if err != nil {
		return "", fmt.Errorf("sentence breaking: %w", err)
	}

	var result []srtBlock
	for _, block := range blocks {
		start, end, err := parseSRTTimeline(block.Timeline)
		if err != nil {
			return "", fmt.Errorf("sentence breaking: %w", err)
		}
		splitBlocks := splitBlockIfNeeded(block, start, end)
		result = append(result, splitBlocks...)
	}

	for i := range result {
		result[i].Number = strconv.Itoa(i + 1)
		result[i].Raw = buildBlockRaw(result[i].Number, result[i].Timeline, extractTextLines(result[i].Raw))
	}

	return joinSRTBlocks(result), nil
}

func splitBlockIfNeeded(block srtBlock, start, end time.Duration) []srtBlock {
	text := strings.Join(extractTextLines(block.Raw), " ")
	duration := end - start
	chars := charCount(text)

	if duration <= maxDurationPerEntry && chars <= maxCharsPerEntry {
		return []srtBlock{block}
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
		return []srtBlock{block}
	}

	return splitBlockEvenly(text, start, end, numParts)
}

func splitBlockEvenly(text string, start, end time.Duration, numParts int) []srtBlock {
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

	blocks := make([]srtBlock, len(result))
	for i, seg := range result {
		blocks[i] = srtBlock{
			Timeline: formatSRTTimeline(seg.start, seg.end),
			Raw:      "0\n" + formatSRTTimeline(seg.start, seg.end) + "\n" + seg.text,
		}
	}
	return blocks
}

func applySentenceBreaking(srtPath string) error {
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

func charCount(s string) int {
	return len([]rune(s))
}

func parseSRTTimeline(timeline string) (time.Duration, time.Duration, error) {
	parts := strings.SplitN(timeline, " --> ", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid timeline: %s", timeline)
	}
	start, err := parseSRTTime(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	end, err := parseSRTTime(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func parseSRTTime(s string) (time.Duration, error) {
	s = strings.ReplaceAll(s, ".", ",")
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid srt time: %s", s)
	}
	timeParts := strings.SplitN(parts[0], ":", 3)
	if len(timeParts) != 3 {
		return 0, fmt.Errorf("invalid srt time: %s", s)
	}
	h, _ := strconv.Atoi(timeParts[0])
	m, _ := strconv.Atoi(timeParts[1])
	sec, _ := strconv.Atoi(timeParts[2])
	millis, _ := strconv.Atoi((parts[1] + "000")[:3])

	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(millis)*time.Millisecond, nil
}

func formatSRTTimeline(start, end time.Duration) string {
	return formatSRTTime(start) + " --> " + formatSRTTime(end)
}

func formatSRTTime(d time.Duration) string {
	ms := d.Milliseconds()
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, millis)
}

func extractTextLines(raw string) []string {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 {
		return lines
	}
	var textLines []string
	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		if line != "" {
			textLines = append(textLines, line)
		}
	}
	return textLines
}

func buildBlockRaw(number, timeline string, lines []string) string {
	return number + "\n" + timeline + "\n" + strings.Join(lines, "\n")
}

func joinSRTBlocks(blocks []srtBlock) string {
	parts := make([]string, len(blocks))
	for i, b := range blocks {
		parts[i] = b.Raw
	}
	return strings.Join(parts, "\n\n") + "\n"
}
