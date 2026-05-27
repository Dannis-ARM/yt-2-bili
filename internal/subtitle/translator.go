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
	translationMaxAttempts          = 2
	translationRequestTimeout       = 60 * time.Second
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
		part, err := t.translateSRTBatch(ctx, batch)
		if err != nil {
			return "", err
		}
		translated = append(translated, strings.TrimSpace(part))
	}
	return strings.Join(translated, "\n\n") + "\n", nil
}

func (t *LLMTranslator) translateSRTBatch(ctx context.Context, srt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < translationMaxAttempts; attempt++ {
		translated, err := t.translateSRTBatchOnce(ctx, srt)
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

func (t *LLMTranslator) translateSRTBatchOnce(ctx context.Context, srt string) (string, error) {
	switch t.provider {
	case "anthropic":
		return t.translateAnthropic(ctx, srt)
	default:
		return t.translateOpenAI(ctx, srt)
	}
}

// ---- OpenAI-compatible API ----

func (t *LLMTranslator) translateOpenAI(ctx context.Context, srt string) (string, error) {
	body := openAIChatRequest{
		Model:  t.model,
		Stream: true,
		Messages: []openAIMessage{
			{Role: "system", Content: `You are an SRT translator. Translate the subtitle text in each SRT block to Simplified Chinese.

CRITICAL RULES — VIOLATION WILL CAUSE PARSING FAILURE:
1. Output ONLY the translated SRT. No greetings, explanations, or markdown fences.
2. Keep every block number and timeline EXACTLY unchanged — copy them verbatim.
3. Keep exactly the same number of blocks. Each block: number line, timeline line, one or more text lines.
4. Separate blocks with exactly ONE blank line. Do NOT merge or split blocks.
5. Translate only the text lines inside each block. Preserve line breaks within text.
6. No trailing commentary, no leading text before the first block number.
7. Be CONCISE: keep translated text roughly the same length as the original. Do NOT add words.`},
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

	fmt.Fprintf(os.Stderr, "POST %s (model=%s)... ", t.baseURL+"/chat/completions", t.model)
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

	translated, err := readOpenAIStream(resp.Body)
	if err != nil {
		return "", err
	}
	if err := validateTranslatedSRT(srt, translated); err != nil {
		return "", err
	}
	return translated, nil
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

func readOpenAIStream(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	var translated strings.Builder
	chunks := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			fmt.Fprintf(os.Stderr, " (%d chunks)", chunks)
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
		if chunks%10 == 0 {
			fmt.Fprint(os.Stderr, ".")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return translated.String(), nil
}

// ---- Anthropic Messages API ----

func (t *LLMTranslator) translateAnthropic(ctx context.Context, srt string) (string, error) {
	body := anthropicRequest{
		Model:      t.model,
		MaxTokens:  20000,
		Thinking:   &anthropicThinking{Type: "disabled"},
		System:     `You are an SRT translator. Translate the subtitle text in each SRT block to Simplified Chinese.

CRITICAL RULES — VIOLATION WILL CAUSE PARSING FAILURE:
1. Output ONLY the translated SRT. No greetings, explanations, or markdown fences.
2. Keep every block number and timeline EXACTLY unchanged — copy them verbatim.
3. Keep exactly the same number of blocks. Each block: number line, timeline line, one or more text lines.
4. Separate blocks with exactly ONE blank line. Do NOT merge or split blocks.
5. Translate only the text lines inside each block. Preserve line breaks within text.
6. No trailing commentary, no leading text before the first block number.
7. Be CONCISE: keep translated text roughly the same length as the original. Do NOT add words.`,
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

	fmt.Fprintf(os.Stderr, "POST %s (model=%s)... ", t.baseURL+"/v1/messages", t.model)
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

	translated, err := readAnthropicStream(resp.Body)
	if err != nil {
		return "", err
	}
	dumpTranslation(translated)
	if err := validateTranslatedSRT(srt, translated); err != nil {
		return "", err
	}
	return translated, nil
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

func readAnthropicStream(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	var translated strings.Builder
	chunks := 0
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
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type == "text_delta" {
			translated.WriteString(event.Delta.Text)
			chunks++
			if chunks%10 == 0 {
				fmt.Fprint(os.Stderr, ".")
			}
		}
		if event.Type == "message_stop" {
			fmt.Fprintf(os.Stderr, " (%d chunks)", chunks)
			return translated.String(), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
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
	if len(sourceBlocks) != len(translatedBlocks) {
		return fmt.Errorf("translated subtitle block count mismatch: source=%d translated=%d", len(sourceBlocks), len(translatedBlocks))
	}
	for i := range sourceBlocks {
		if sourceBlocks[i].Number != translatedBlocks[i].Number {
			return fmt.Errorf("translated subtitle number mismatch at block %d", i+1)
		}
		if sourceBlocks[i].Timeline != translatedBlocks[i].Timeline {
			return fmt.Errorf("translated subtitle timeline mismatch at block %s", sourceBlocks[i].Number)
		}
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

func dumpTranslation(content string) {
	tmp, err := os.CreateTemp("", "yt-2-bili-translated-*.srt")
	if err != nil {
		return
	}
	defer tmp.Close()
	tmp.WriteString(content)
	fmt.Fprintf(os.Stderr, "Translation dumped to: %s\n", tmp.Name())
}
