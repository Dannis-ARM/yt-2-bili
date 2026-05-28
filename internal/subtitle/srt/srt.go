// Package srt provides parsing and writing for SRT subtitle files.
package srt

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Block represents a single SRT subtitle block.
type Block struct {
	Number string
	Start  time.Duration
	End    time.Duration
	Text   string
}

// Parse parses SRT content into blocks.
func Parse(content string) ([]Block, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(content), "\r\n", "\n")
	rawBlocks := strings.Split(normalized, "\n\n")
	blocks := make([]Block, 0, len(rawBlocks))

	for _, raw := range rawBlocks {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		lines := strings.Split(raw, "\n")
		if len(lines) < 3 {
			continue
		}

		number := strings.TrimSpace(lines[0])
		timeline := strings.TrimSpace(lines[1])
		text := strings.Join(lines[2:], "\n")

		start, end, err := parseSRTTimeline(timeline)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, Block{
			Number: number,
			Start:  start,
			End:    end,
			Text:   text,
		})
	}

	return blocks, nil
}

// ParseFile reads and parses an SRT file.
func ParseFile(path string) ([]Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

// Write writes blocks to an SRT file.
func Write(path string, blocks []Block) error {
	content := Format(blocks)
	return os.WriteFile(path, []byte(content), 0o644)
}

// Format formats blocks into SRT content.
func Format(blocks []Block) string {
	var sb strings.Builder
	for i, block := range blocks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		num := block.Number
		if num == "" {
			num = fmt.Sprintf("%d", i+1)
		}
		fmt.Fprintf(&sb, "%s\n%s\n%s", num, formatSRTTimeline(block.Start, block.End), block.Text)
	}
	sb.WriteString("\n")
	return sb.String()
}

// CountBlocks counts the number of blocks in an SRT file.
func CountBlocks(path string) (int, error) {
	blocks, err := ParseFile(path)
	if err != nil {
		return 0, err
	}
	return len(blocks), nil
}

func parseSRTTimeline(timeline string) (start, end time.Duration, err error) {
	parts := strings.SplitN(timeline, " --> ", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid timeline: %s", timeline)
	}
	start, err = parseSRTTime(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	end, err = parseSRTTime(strings.TrimSpace(parts[1]))
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
	h, err := strconv.Atoi(timeParts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(timeParts[1])
	if err != nil {
		return 0, err
	}
	sec, err := strconv.Atoi(timeParts[2])
	if err != nil {
		return 0, err
	}
	millis, err := strconv.Atoi((parts[1] + "000")[:3])
	if err != nil {
		return 0, err
	}

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
