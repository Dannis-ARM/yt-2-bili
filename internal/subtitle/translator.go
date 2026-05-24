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
	"strings"
)

const (
	defaultTranslationBatchCharLimit = 120000
	translationMaxAttempts          = 3
)

type LLMTranslatorOptions struct {
	BaseURL        string
	APIKey         string
	Model          string
	Client         *http.Client
	BatchCharLimit int
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
		client = http.DefaultClient
	}
	batchCharLimit := opts.BatchCharLimit
	if batchCharLimit == 0 {
		batchCharLimit = defaultTranslationBatchCharLimit
	}
	return &LLMTranslator{
		baseURL:        strings.TrimRight(opts.BaseURL, "/"),
		apiKey:         opts.APIKey,
		model:          opts.Model,
		client:         client,
		batchCharLimit: batchCharLimit,
	}
}

func (t *LLMTranslator) TranslateSRT(ctx context.Context, srt string) (string, error) {
	batches, err := splitSRTBatches(srt, t.batchCharLimit)
	if err != nil {
		return "", err
	}
	translated := make([]string, 0, len(batches))
	for _, batch := range batches {
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
	body := chatCompletionRequest{
		Model:  t.model,
		Stream: true,
		Messages: []chatMessage{
			{Role: "system", Content: "Translate SRT subtitles to Simplified Chinese. Return only valid SRT."},
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

	translated, err := readStreamingContent(resp.Body)
	if err != nil {
		return "", err
	}
	if err := validateTranslatedSRT(srt, translated); err != nil {
		return "", err
	}
	return translated, nil
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Stream   bool          `json:"stream"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func readStreamingContent(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	var translated strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return translated.String(), nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", err
		}
		for _, choice := range chunk.Choices {
			translated.WriteString(choice.Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return translated.String(), nil
}

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
	if normalized == "" {
		return nil, fmt.Errorf("empty subtitle")
	}
	rawBlocks := strings.Split(normalized, "\n\n")
	blocks := make([]srtBlock, 0, len(rawBlocks))
	for _, raw := range rawBlocks {
		raw = strings.TrimSpace(raw)
		lines := strings.Split(raw, "\n")
		if len(lines) < 3 {
			return nil, fmt.Errorf("subtitle block has fewer than three lines")
		}
		number := strings.TrimSpace(lines[0])
		timeline := strings.TrimSpace(lines[1])
		if number == "" || !strings.Contains(timeline, "-->") {
			return nil, fmt.Errorf("subtitle block has invalid header")
		}
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
