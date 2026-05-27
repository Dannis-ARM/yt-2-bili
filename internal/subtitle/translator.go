package subtitle

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	defaultTranslationBatchCharLimit = 120000
	translationMaxAttempts          = 1
	translationRequestTimeout       = 600 * time.Second
	anthropicVersion                = "2023-06-01"
)

type LLMTranslatorOptions struct {
	BaseURL        string
	APIKey         string
	Model          string
	Client         *http.Client
	BatchCharLimit int
	Provider       string // "" or "openai" for OpenAI API; "anthropic" for Anthropic Messages API
}

type Translator interface {
	TranslateSRT(ctx context.Context, srt string) (string, error)
	TranslateText(ctx context.Context, text string) (string, error)
}

type LLMTranslator struct {
	baseURL        string
	apiKey         string
	model          string
	client         *http.Client
	batchCharLimit int
	provider       string
}

type nonRetryableError struct {
	err error
}

func (e nonRetryableError) Error() string {
	return e.err.Error()
}

func (e nonRetryableError) Unwrap() error {
	return e.err
}

func NewLLMTranslator(opts LLMTranslatorOptions) *LLMTranslator {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: translationRequestTimeout}
	}
	batchCharLimit := opts.BatchCharLimit
	if batchCharLimit == 0 {
		batchCharLimit = defaultTranslationBatchCharLimit
	}
	provider := opts.Provider
	if provider == "" {
		provider = "openai"
	}
	return &LLMTranslator{
		baseURL:        strings.TrimRight(opts.BaseURL, "/"),
		apiKey:         opts.APIKey,
		model:          opts.Model,
		client:         client,
		batchCharLimit: batchCharLimit,
		provider:       provider,
	}
}

func (t *LLMTranslator) TranslateSRT(ctx context.Context, srt string) (string, error) {
	blocks, err := parseSRTBlocks(srt)
	if err != nil {
		return "", err
	}

	batches := batchSRTBlocks(blocks, t.batchCharLimit)
	fmt.Fprintf(os.Stderr, "Translating %d block(s) in %d batch(es)...\n", len(blocks), len(batches))

	texts := make([]string, 0, len(blocks))
	for i, batch := range batches {
		fmt.Fprintf(os.Stderr, "Translating batch %d/%d...\n", i+1, len(batches))
		input := buildTextInput(batch)
		output, err := t.translateWithPrompt(ctx, input, systemPromptSRT, "SRT")
		if err != nil {
			return "", err
		}
		parsed, err := parseTextOutput(output, len(batch))
		if err != nil {
			return "", err
		}
		texts = append(texts, parsed...)
	}

	return reconstructSRT(blocks, texts), nil
}

func (t *LLMTranslator) TranslateText(ctx context.Context, text string) (string, error) {
	label := "text:" + truncateForLabel(text, 30)
	return t.translateWithPrompt(ctx, text, systemPromptText, label)
}

func (t *LLMTranslator) translateWithPrompt(ctx context.Context, input string, systemPrompt string, label string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < translationMaxAttempts; attempt++ {
		var output string
		var err error
		switch t.provider {
		case "anthropic":
			output, err = t.translateAnthropic(ctx, input, systemPrompt, label)
		default:
			output, err = t.translateOpenAI(ctx, input, systemPrompt, label)
		}
		if err == nil {
			return output, nil
		}
		lastErr = err
		var nonRetryable nonRetryableError
		if errors.As(err, &nonRetryable) {
			break
		}
	}
	return "", lastErr
}

const (
	systemPromptSRT = `Translate each numbered entry to Simplified Chinese. Keep the [N] markers exactly unchanged and in the same order. Translate only the text after each marker. Output ONLY the translated entries with markers. No greetings, explanations, or markdown fences. Be concise.`

	systemPromptText = `You are a translator. Translate the given text to Simplified Chinese.

CRITICAL RULES:
1. Output ONLY the translated text. No greetings, explanations, or markdown fences.
2. Preserve the original formatting (line breaks, etc.).
3. Be natural and fluent in Chinese.`
)

// ---- OpenAI-compatible API ----

func (t *LLMTranslator) translateOpenAI(ctx context.Context, input string, systemPrompt string, label string) (string, error) {
	body := openAIChatRequest{
		Model:  t.model,
		Stream: true,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "POST [%s] %s... ", label, t.baseURL+"/chat/completions")
	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("llm translation failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return "", nonRetryableError{err: err}
		}
		return "", err
	}

	return readOpenAIStream(resp.Body, label)
}

type openAIChatRequest struct {
	Model    string         `json:"model"`
	Stream   bool           `json:"stream"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func readOpenAIStream(r io.Reader, label string) (string, error) {
	scanner := bufio.NewScanner(r)
	var translated strings.Builder
	chunks := 0
	lastProgress := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			fmt.Fprintf(os.Stderr, " done (%d chars, %d chunks)", translated.Len(), chunks)
			return translated.String(), nil
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", err
		}
		for _, choice := range chunk.Choices {
			translated.WriteString(choice.Delta.Content)
		}
		chunks++
		if current := translated.Len(); current-lastProgress >= 1000 {
			fmt.Fprintf(os.Stderr, " (%d chars)", current)
			lastProgress = current
		}
	}
	if err := scanner.Err(); err != nil {
		if translated.Len() > 0 {
			fmt.Fprintf(os.Stderr, " (%d chars, PARTIAL)", translated.Len())
			dumpTranslation(translated.String())
		}
		return "", fmt.Errorf("[%s] %w", label, err)
	}
	fmt.Fprintf(os.Stderr, " done (%d chars, %d chunks)", translated.Len(), chunks)
	return translated.String(), nil
}

// ---- Anthropic Messages API ----

func (t *LLMTranslator) translateAnthropic(ctx context.Context, input string, systemPrompt string, label string) (string, error) {
	body := anthropicRequest{
		Model:     t.model,
		MaxTokens: 100000,
		Thinking:  &anthropicThinking{Type: "disabled"},
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: input},
		},
		Stream: true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", t.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "POST [%s] %s... ", label, t.baseURL+"/v1/messages")
	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("llm translation failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return "", nonRetryableError{err: err}
		}
		return "", err
	}

	return readAnthropicStream(resp.Body, label)
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
	Thinking  *anthropicThinking `json:"thinking,omitempty"`
}

type anthropicThinking struct {
	Type string `json:"type"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicSSEEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func readAnthropicStream(r io.Reader, label string) (string, error) {
	scanner := bufio.NewScanner(r)
	var translated strings.Builder
	chunks := 0
	lastProgress := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Type == "error" {
			return "", fmt.Errorf("[%s] API error: %s", label, data)
		}
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type == "text_delta" {
			translated.WriteString(event.Delta.Text)
			chunks++
			if current := translated.Len(); current-lastProgress >= 1000 {
				fmt.Fprintf(os.Stderr, " (%d chars)", current)
				lastProgress = current
			}
		}
		if event.Type == "message_stop" {
			fmt.Fprintf(os.Stderr, " done (%d chars, %d chunks)", translated.Len(), chunks)
			return translated.String(), nil
		}
	}
	if err := scanner.Err(); err != nil {
		if translated.Len() > 0 {
			fmt.Fprintf(os.Stderr, " (%d chars, PARTIAL)", translated.Len())
			dumpTranslation(translated.String())
		}
		return "", fmt.Errorf("[%s] %w", label, err)
	}
	fmt.Fprintf(os.Stderr, " done (%d chars, %d chunks)", translated.Len(), chunks)
	return translated.String(), nil
}

// ---- SRT text-only helpers ----

func batchSRTBlocks(blocks []srtBlock, limit int) [][]srtBlock {
	var batches [][]srtBlock
	var current []srtBlock
	currentLen := 0
	for _, block := range blocks {
		if len(block.Text) > limit {
			// Single block exceeds limit — still process it alone
			batches = append(batches, []srtBlock{block})
			continue
		}
		sep := 0
		if len(current) > 0 {
			sep = 1 // newline between entries
		}
		if len(current) > 0 && currentLen+sep+len(block.Text) > limit {
			batches = append(batches, current)
			current = nil
			currentLen = 0
			sep = 0
		}
		current = append(current, block)
		currentLen += sep + len(block.Text)
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func buildTextInput(blocks []srtBlock) string {
	var b strings.Builder
	for i, block := range blocks {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, block.Text)
	}
	return b.String()
}

var textMarkerRe = regexp.MustCompile(`(?m)^\[(\d+)\]\s*`)

func parseTextOutput(output string, expectedCount int) ([]string, error) {
	matches := textMarkerRe.FindAllStringSubmatchIndex(output, -1)
	if len(matches) != expectedCount {
		return nil, fmt.Errorf("expected %d translated entries, got %d", expectedCount, len(matches))
	}
	texts := make([]string, 0, len(matches))
	for i, m := range matches {
		textStart := m[1]
		textEnd := len(output)
		if i+1 < len(matches) {
			textEnd = matches[i+1][0]
		}
		texts = append(texts, strings.TrimSpace(output[textStart:textEnd]))
	}
	return texts, nil
}

func reconstructSRT(blocks []srtBlock, texts []string) string {
	var out strings.Builder
	for i, block := range blocks {
		if i > 0 {
			out.WriteString("\n\n")
		}
		fmt.Fprintf(&out, "%s\n%s\n%s", block.Number, block.Timeline, texts[i])
	}
	out.WriteString("\n")
	return out.String()
}

// ---- Shared ----

type srtBlock struct {
	Number   string
	Timeline string
	Text     string // text lines only (everything after timeline)
}

func validateTranslatedSRT(source, translated string) error {
	sourceBlocks, err := parseSRTBlocks(source)
	if err != nil {
		return fmt.Errorf("source subtitle is invalid: %w", err)
	}
	translatedBlocks, err := parseSRTBlocks(translated)
	if err != nil {
		return fmt.Errorf("translated subtitle is invalid: %w", err)
	}

	sourceTimelines := make(map[string]bool, len(sourceBlocks))
	for _, b := range sourceBlocks {
		sourceTimelines[b.Timeline] = true
	}

	for _, b := range translatedBlocks {
		if !sourceTimelines[b.Timeline] {
			return fmt.Errorf("translated timeline %q not found in source", b.Timeline)
		}
	}

	if len(sourceBlocks) != len(translatedBlocks) {
		fmt.Fprintf(os.Stderr, "Block count differs (source=%d translated=%d), continuing\n", len(sourceBlocks), len(translatedBlocks))
	}

	return nil
}

func parseSRTBlocks(srt string) ([]srtBlock, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(srt), "\r\n", "\n")
	rawBlocks := strings.Split(normalized, "\n\n")
	blocks := make([]srtBlock, 0, len(rawBlocks))
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
		blocks = append(blocks, srtBlock{Number: number, Timeline: timeline, Text: text})
	}
	return blocks, nil
}

func truncateForLabel(s string, maxLen int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", " "), "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func dumpTranslation(content string) {
	tmp, err := os.CreateTemp("", "yt-2-bili-translated-*.srt")
	if err != nil {
		return
	}
	defer tmp.Close()
	tmp.WriteString(content)
	fmt.Fprintf(os.Stderr, "Translation dumped to: %s\n", tmp.Name())
}
