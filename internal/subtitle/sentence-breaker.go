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

var sentenceEndPunct = map[rune]bool{'.': true, '!': true, '?': true}

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
		splitBlocks := breakBlock(block, start, end)
		result = append(result, splitBlocks...)
	}

	for i := range result {
		result[i].Number = strconv.Itoa(i + 1)
		result[i].Raw = buildBlockRaw(result[i].Number, result[i].Timeline, extractTextLines(result[i].Raw))
	}

	return joinSRTBlocks(result), nil
}

func breakBlock(block srtBlock, start, end time.Duration) []srtBlock {
	fullText := strings.Join(extractTextLines(block.Raw), " ")
	duration := end - start
	if duration <= 0 {
		duration = 1
	}

	if duration <= maxDurationPerEntry && charCount(fullText) <= maxCharsPerEntry {
		return []srtBlock{block}
	}

	segments := splitBySentenceEnd(fullText)
	segments = splitLongByChars(segments)

	totalChars := 0
	for _, seg := range segments {
		totalChars += seg.chars
	}
	if totalChars == 0 {
		return []srtBlock{block}
	}

	timed := allocateTimecodes(segments, start, end, totalChars)
	timed = splitLongByDuration(timed)

	result := make([]srtBlock, len(timed))
	for i, t := range timed {
		result[i] = srtBlock{
			Timeline: formatSRTTimeline(t.start, t.end),
			Raw:      "0\n" + formatSRTTimeline(t.start, t.end) + "\n" + t.text,
		}
	}
	return result
}

type textSegment struct {
	text  string
	chars int
}

type timedSegment struct {
	text  string
	start time.Duration
	end   time.Duration
}

func splitBySentenceEnd(text string) []textSegment {
	var segments []textSegment
	current := strings.Builder{}
	runes := []rune(text)

	for i, r := range runes {
		current.WriteRune(r)
		if sentenceEndPunct[r] && i+1 < len(runes) && runes[i+1] == ' ' {
			segments = append(segments, trimmedSegment(current.String()))
			current.Reset()
		}
	}
	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		segments = append(segments, trimmedSegment(remaining))
	}
	return segments
}

func trimmedSegment(s string) textSegment {
	t := strings.TrimSpace(s)
	return textSegment{text: t, chars: charCount(t)}
}

func splitLongByChars(segments []textSegment) []textSegment {
	var result []textSegment
	for _, seg := range segments {
		if seg.chars == 0 {
			continue
		}
		if seg.chars <= maxCharsPerEntry {
			result = append(result, seg)
			continue
		}
		result = append(result, hardSplit(seg)...)
	}
	return result
}

func hardSplit(seg textSegment) []textSegment {
	var result []textSegment
	remaining := seg.text

	for charCount(remaining) > maxCharsPerEntry {
		splitAt := findBestSplit(remaining)
		runes := []rune(remaining)
		if splitAt <= 0 || splitAt > len(runes) {
			splitAt = maxCharsPerEntry
			if splitAt > len(runes) {
				splitAt = len(runes)
			}
		}
		left := strings.TrimSpace(string(runes[:splitAt]))
		remaining = strings.TrimSpace(string(runes[splitAt:]))
		if left != "" {
			result = append(result, trimmedSegment(left))
		}
	}
	if t := strings.TrimSpace(remaining); t != "" {
		result = append(result, trimmedSegment(t))
	}
	return result
}

func findBestSplit(text string) int {
	runes := []rune(text)
	limit := maxCharsPerEntry
	if limit > len(runes) {
		limit = len(runes)
	}

	searchStart := limit / 2
	for i := limit - 1; i >= searchStart; i-- {
		if runes[i] == ',' && i+1 < len(runes) && runes[i+1] == ' ' {
			return i + 2
		}
		if runes[i] == ',' {
			return i + 1
		}
	}
	for i := limit - 1; i >= searchStart; i-- {
		if runes[i] == ' ' {
			return i + 1
		}
	}
	return 0
}

func allocateTimecodes(segments []textSegment, start, end time.Duration, totalChars int) []timedSegment {
	result := make([]timedSegment, 0, len(segments))
	current := start
	duration := end - start

	for i, seg := range segments {
		if seg.chars == 0 {
			continue
		}
		segDuration := time.Duration(float64(duration) * float64(seg.chars) / float64(totalChars))
		segEnd := current + segDuration
		if i == len(segments)-1 {
			segEnd = end
		}
		if segEnd > end {
			segEnd = end
		}
		result = append(result, timedSegment{text: seg.text, start: current, end: segEnd})
		current = segEnd
	}
	return result
}

func splitLongByDuration(segments []timedSegment) []timedSegment {
	var result []timedSegment
	for _, seg := range segments {
		duration := seg.end - seg.start
		if duration <= maxDurationPerEntry || charCount(seg.text) <= 1 {
			result = append(result, seg)
			continue
		}
		result = append(result, splitByDuration(seg)...)
	}
	return result
}

func splitByDuration(seg timedSegment) []timedSegment {
	duration := seg.end - seg.start
	n := int(math.Ceil(float64(duration) / float64(maxDurationPerEntry)))
	subDuration := duration / time.Duration(n)
	runes := []rune(seg.text)
	charsPerPart := (len(runes) + n - 1) / n

	var result []timedSegment
	current := seg.start
	for i := 0; i < n; i++ {
		charStart := i * charsPerPart
		charEnd := charStart + charsPerPart
		if charEnd > len(runes) {
			charEnd = len(runes)
		}
		partText := strings.TrimSpace(string(runes[charStart:charEnd]))
		if partText == "" {
			continue
		}
		partEnd := current + subDuration
		if i == n-1 {
			partEnd = seg.end
		}
		result = append(result, timedSegment{text: partText, start: current, end: partEnd})
		current = partEnd
	}
	return result
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
