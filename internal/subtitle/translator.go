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
	batches, err := splitSRTBatches(srt, t.batchCharLimit)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "Translating in %d batch(es)...\n", len(batches))
	translated := make([]string, 0, len(batches))
	for i, batch := range batches {
		fmt.Fprintf(os.Stderr, "Translating batch %d/%d...\n", i+1, len(batches))
		part, err := t.translateSRTBatch(ctx, batch, "SRT")
		if err != nil {
			return "", err
		}
		translated = append(translated, strings.TrimSpace(part))
	}
	return strings.Join(translated, "\n\n") + "\n", nil
}

func (t *LLMTranslator) TranslateText(ctx context.Context, text string) (string, error) {
	label := "text:" + truncateForLabel(text, 30)
	for attempt := 0; attempt < translationMaxAttempts; attempt++ {
		translated, err := t.translateTextOnce(ctx, text, label)
		if err == nil {
			return translated, nil
		}
		var nonRetryable nonRetryableError
		if errors.As(err, &nonRetryable) {
			return "", err
		}
	}
	return "", fmt.Errorf("translation failed after %d attempts", translationMaxAttempts)
}

func (t *LLMTranslator) translateTextOnce(ctx context.Context, text string, label string) (string, error) {
	switch t.provider {
	case "anthropic":
		return t.translateTextAnthropic(ctx, text, label)
	default:
		return t.translateTextOpenAI(ctx, text, label)
	}
}

func (t *LLMTranslator) translateSRTBatch(ctx context.Context, srt string, label string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < translationMaxAttempts; attempt++ {
		translated, err := t.translateSRTBatchOnce(ctx, srt, label)
		if err == nil {
			return translated, nil
		}
		lastErr = err
		var nonRetryable nonRetryableError
		if errors.As(err, &nonRetryable) {
			break
		}
	}
	return "", lastErr
}

func (t *LLMTranslator) translateSRTBatchOnce(ctx context.Context, srt string, label string) (string, error) {
	switch t.provider {
	case "anthropic":
		return t.translateAnthropic(ctx, srt, label)
	default:
		return t.translateOpenAI(ctx, srt, label)
	}
}

const (
	systemPromptSRT = `You are an SRT translator. Translate the subtitle text in each SRT block to Simplified Chinese.

CRITICAL RULES — VIOLATION WILL CAUSE PARSING FAILURE:
1. Output ONLY the translated SRT. No greetings, explanations, or markdown fences.
2. Keep every block number and timeline EXACTLY unchanged — copy them verbatim.
3. Keep exactly the same number of blocks. Each block: number line, timeline line, one or more text lines.
4. Separate blocks with exactly ONE blank line. Do NOT merge or split blocks.
5. Translate only the text lines inside each block. Preserve line breaks within text.
6. No trailing commentary, no leading text before the first block number.
7. Be CONCISE: keep translated text roughly the same length as the original. Do NOT add words.`

	systemPromptText = `You are a translator. Translate the given text to Simplified Chinese.

CRITICAL RULES:
1. Output ONLY the translated text. No greetings, explanations, or markdown fences.
2. Preserve the original formatting (line breaks, etc.).
3. Be natural and fluent in Chinese.`
)

// ---- OpenAI-compatible API ----

func (t *LLMTranslator) translateOpenAI(ctx context.Context, srt string, label string) (string, error) {
	body := openAIChatRequest{
		Model:  t.model,
		Stream: true,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPromptSRT},
			{Role: "user", Content: srt},
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

	translated, err := readOpenAIStream(resp.Body, label)
	if err != nil {
		return "", err
	}
	if err := validateTranslatedSRT(srt, translated); err != nil {
		return "", nonRetryableError{err: err}
	}
	return translated, nil
}

func (t *LLMTranslator) translateTextOpenAI(ctx context.Context, text string, label string) (string, error) {
	body := openAIChatRequest{
		Model:  t.model,
		Stream: true,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPromptText},
			{Role: "user", Content: text},
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

	translated, err := readOpenAIStream(resp.Body, label)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(translated), nil
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

func (t *LLMTranslator) translateAnthropic(ctx context.Context, srt string, label string) (string, error) {
	body := anthropicRequest{
		Model:      t.model,
		MaxTokens:  100000,
		Thinking:   &anthropicThinking{Type: "disabled"},
		System:   systemPromptSRT,
		Messages: []anthropicMessage{
			{Role: "user", Content: srt},
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

	translated, err := readAnthropicStream(resp.Body, label)
	if err != nil {
		return "", err
	}
	if err := validateTranslatedSRT(srt, translated); err != nil {
		return "", nonRetryableError{err: err}
	}
	return translated, nil
}

func (t *LLMTranslator) translateTextAnthropic(ctx context.Context, text string, label string) (string, error) {
	body := anthropicRequest{
		Model:      t.model,
		MaxTokens:  32000,
		Thinking:   &anthropicThinking{Type: "disabled"},
		System:   systemPromptText,
		Messages: []anthropicMessage{
			{Role: "user", Content: text},
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

	translated, err := readAnthropicStream(resp.Body, label)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(translated), nil
}

type anthropicRequest struct {
	Model      string             `json:"model"`
	MaxTokens  int                `json:"max_tokens"`
	System     string             `json:"system"`
	Messages   []anthropicMessage `json:"messages"`
	Stream     bool               `json:"stream"`
	Thinking   *anthropicThinking `json:"thinking,omitempty"`
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

// ---- Shared ----

type srtBlock struct {
	Number   string
	Timeline string
	Raw      string
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

	// Build set of valid source timelines
	sourceTimelines := make(map[string]bool, len(sourceBlocks))
	for _, b := range sourceBlocks {
		sourceTimelines[b.Timeline] = true
	}

	// Only validate timeline integrity — block count and numbering are allowed to differ
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
		if len(lines) < 2 {
			continue
		}
		number := strings.TrimSpace(lines[0])
		timeline := strings.TrimSpace(lines[1])
		blocks = append(blocks, srtBlock{Number: number, Timeline: timeline, Raw: raw})
	}
	return blocks, nil
}

func splitSRTBatches(srt string, limit int) ([]string, error) {
	blocks, err := parseSRTBlocks(srt)
	if err != nil {
		return nil, err
	}
	var batches []string
	var current strings.Builder
	for _, block := range blocks {
		blockText := block.Raw
		if len(blockText) > limit {
			return nil, fmt.Errorf("subtitle block %s exceeds translation batch size", block.Number)
		}
		separator := ""
		if current.Len() > 0 {
			separator = "\n\n"
		}
		if current.Len() > 0 && current.Len()+len(separator)+len(blockText) > limit {
			batches = append(batches, current.String())
			current.Reset()
			separator = ""
		}
		current.WriteString(separator)
		current.WriteString(blockText)
	}
	if current.Len() > 0 {
		batches = append(batches, current.String())
	}
	return batches, nil
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
