package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run test-ark-simple.go <srt-file>")
		os.Exit(1)
	}

	srtData, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read SRT file: %v\n", err)
		os.Exit(1)
	}
	srt := string(srtData)

	token := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Set ANTHROPIC_AUTH_TOKEN first")
		os.Exit(1)
	}

	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/anthropic"
	}

	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "deepseek-v4-flash"
	}

	systemPrompt := `You are an SRT translator. Translate the subtitle text in each SRT block to Simplified Chinese.

CRITICAL RULES — VIOLATION WILL CAUSE PARSING FAILURE:
1. Output ONLY the translated SRT. No greetings, explanations, or markdown fences.
2. Keep every block number and timeline EXACTLY unchanged — copy them verbatim.
3. Keep exactly the same number of blocks. Each block: number line, timeline line, one or more text lines.
4. Separate blocks with exactly ONE blank line. Do NOT merge or split blocks.
5. Translate only the text lines inside each block. Preserve line breaks within text.
6. No trailing commentary, no leading text before the first block number.
7. Be CONCISE: keep translated text roughly the same length as the original. Do NOT add words.`

	fmt.Fprintf(os.Stderr, "SRT file: %s (%d bytes, %d lines)\n", os.Args[1], len(srt), countLines(srt))
	fmt.Fprintf(os.Stderr, "Model: %s\n", model)
	fmt.Fprintf(os.Stderr, "Prompt tokens (est): %d chars\n", len(systemPrompt))
	fmt.Fprintf(os.Stderr, "SRT content: %d chars\n", len(srt))
	fmt.Fprintf(os.Stderr, "---\n")

	// Non-streaming first: quick test
	fmt.Fprintln(os.Stderr, "=== Non-streaming (max_tokens=4096) ===")
	dump := testRequest(baseURL, token, model, systemPrompt, srt, false, 4096)
	if dump != "" {
		fmt.Fprintf(os.Stderr, "Output dumped to: %s\n", dump)
	}

	fmt.Fprintln(os.Stderr)

	// Streaming test
	fmt.Fprintln(os.Stderr, "=== Streaming (max_tokens=2048) ===")
	dump = testRequest(baseURL, token, model, systemPrompt, srt, true, 2048)
	if dump != "" {
		fmt.Fprintf(os.Stderr, "Output dumped to: %s\n", dump)
	}
}

func countLines(s string) int {
	n := 0
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}

func testRequest(baseURL, token, model, systemPrompt, userContent string, stream bool, maxTokens int) string {
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
		"stream": stream,
	}

	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", baseURL+"/v1/messages", strings.NewReader(string(payload)))
	req.Header.Set("x-api-key", token)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	fmt.Fprintf(os.Stderr, "Status: %d (elapsed: %v)\n", resp.StatusCode, time.Since(start).Round(time.Millisecond))

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error body: %s\n", string(respBody))
		return ""
	}

	if stream {
		return readStreamDump(resp.Body)
	}
	return readNonStreamDump(resp.Body)
}

func readNonStreamDump(r io.Reader) string {
	raw, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
		return ""
	}

	// Dump raw response first to see actual format
	fmt.Fprintf(os.Stderr, "Raw response (first 2000 chars):\n%s\n---\n", string(raw[:min(len(raw), 2000)]))

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\nFull raw dumped\n", err)
		return writeDumpRaw(raw, ".json")
	}

	if len(result.Content) == 0 {
		fmt.Fprintln(os.Stderr, "No content — full raw dumped")
		return writeDumpRaw(raw, ".json")
	}

	text := result.Content[0].Text
	fmt.Fprintf(os.Stderr, "Output: %d chars, %d lines\n", len(text), countLines(text))
	return writeDump(text)
}

func readStreamDump(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	var content strings.Builder
	chunks := 0
	eventsDumped := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

		// Dump first 3 SSE events raw for inspection
		if eventsDumped < 3 {
			fmt.Fprintf(os.Stderr, "SSE[%d]: %s\n", eventsDumped, data[:min(len(data), 300)])
			eventsDumped++
		}

		var event struct {
			Type  string `json:"type"`
			Delta *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			ContentBlock *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content_block"`
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		if event.Type == "content_block_delta" && event.Delta != nil {
			content.WriteString(event.Delta.Text)
			chunks++
		} else if event.Type == "content_block_delta" && event.ContentBlock != nil {
			content.WriteString(event.ContentBlock.Text)
			chunks++
		} else if event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Text != "" {
			content.WriteString(event.ContentBlock.Text)
			chunks++
		}
		if chunks > 0 && chunks%50 == 0 {
			fmt.Fprint(os.Stderr, ".")
		}
		if event.Type == "message_stop" {
			break
		}
	}
	fmt.Fprintf(os.Stderr, " (%d chunks)\n", chunks)

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Stream error: %v\n", err)
	}

	text := content.String()
	fmt.Fprintf(os.Stderr, "Output: %d chars, %d lines\n", len(text), countLines(text))
	return writeDump(text)
}

func writeDump(content string) string {
	return writeDumpRaw([]byte(content), ".srt")
}

func writeDumpRaw(data []byte, ext string) string {
	tmp, err := os.CreateTemp("", "yt-2-bili-test-output-*"+ext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Temp file error: %v\n", err)
		return ""
	}
	defer tmp.Close()
	tmp.Write(data)
	return tmp.Name()
}
