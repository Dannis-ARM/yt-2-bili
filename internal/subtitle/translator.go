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
	"strconv"
	"strings"
	"time"

	"github.com/dannis/yt-2-bili/internal/subtitle/srt"
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

func (t *LLMTranslator) TranslateSRT(ctx context.Context, srtContent string) (string, error) {
	blocks, err := srt.Parse(srtContent)
	if err != nil {
		return "", err
	}

	batches := batchSRTBlocks(blocks, t.batchCharLimit)
	fmt.Fprintf(os.Stderr, "Translating %d block(s) in %d batch(es)...\n", len(blocks), len(batches))

	var warnings TranslationWarnings
	texts := make([]string, 0, len(blocks))
	for i, batch := range batches {
		fmt.Fprintf(os.Stderr, "Translating batch %d/%d...\n", i+1, len(batches))
		input := buildTextInput(batch)
		var parsed *ParseResult
		var lastOutput string
		for attempt := 0; attempt < 3; attempt++ {
			label := fmt.Sprintf("SRT-b%d", i+1)
			if attempt > 0 {
				label = fmt.Sprintf("SRT-b%d-retry%d", i+1, attempt)
			}
			output, err := t.translateWithPrompt(ctx, input, systemPromptSRT, label)
			if err != nil {
				return "", err
			}
			lastOutput = output
			parsed, err = parseTextOutput(output, len(batch))
			if err != nil {
				return "", err
			}
			if len(parsed.Issues) == 0 {
				break
			}
			if attempt < 2 {
				fmt.Fprintf(os.Stderr, "Batch %d attempt %d: %d issue(s), retrying...\n", i+1, attempt+1, len(parsed.Issues))
				input = buildRetryPrompt(batch, parsed, output)
			}
		}
		if len(parsed.Issues) > 0 {
			warnings.add(parsed.Issues)
		}
		for j, t := range parsed.Texts {
			if t == "" && j < len(batch) {
				parsed.Texts[j] = batch[j].Text
			}
		}
		texts = append(texts, parsed.Texts...)
		_ = lastOutput
	}

	if s := warnings.summary(); s != "" {
		fmt.Fprintf(os.Stderr, "%s\n", s)
		for _, d := range warnings.Details {
			fmt.Fprintf(os.Stderr, "  - %s\n", d)
		}
	}

	return reconstructSRT(blocks, texts), nil
}

const maxRetryPromptChars = 200000

// buildRetryPrompt constructs a correction prompt that includes the previous LLM
// output so the model can patch rather than re-translate from scratch.
func buildRetryPrompt(blocks []srt.Block, prevResult *ParseResult, prevOutput string) string {
	var sb strings.Builder
	sb.WriteString("Your previous translation had the following issues:\n")
	for _, issue := range prevResult.Issues {
		fmt.Fprintf(&sb, "- %s\n", issue.Details)
	}
	sb.WriteString("\nHere is your previous output for reference:\n\n")
	sb.WriteString(prevOutput)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString("Please fix the issues above and output the COMPLETE translation again.\n")
	sb.WriteString("CRITICAL RULES:\n")
	sb.WriteString("1. Keep ALL [N] markers exactly unchanged, from [1] to [")
	fmt.Fprintf(&sb, "%d", len(blocks))
	sb.WriteString("], in order\n")
	sb.WriteString("2. Do NOT skip or reorder any markers\n")
	sb.WriteString("3. Translate EVERY entry with its actual meaning — NEVER use placeholder text like \"这里是第N条内容\"\n")
	sb.WriteString("4. Output ONLY the fixed translation with markers. No explanations.\n")

	result := sb.String()
	if len(result) > maxRetryPromptChars {
		result = result[:maxRetryPromptChars]
	}
	return result
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
	systemPromptSRT = `Translate each numbered entry to Simplified Chinese. Keep the [N] markers exactly unchanged and in the same order. Translate only the text after each marker. Output ONLY the translated entries with markers. No greetings, explanations, or markdown fences. Be concise.

CRITICAL: Translate EVERY entry faithfully. Never use placeholder text like "这里是第N条内容" or "第N条内容" — each translation must reflect the actual meaning of the source text.`

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
	Model    string
	Stream   bool
	Messages []openAIMessage
}

type openAIMessage struct {
	Role    string
	Content string
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string
		}
	}
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
			fmt.Fprintf(os.Stderr, "done (%d chars, %d chunks)", translated.Len(), chunks)
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
	fmt.Fprintf(os.Stderr, "done (%d chars, %d chunks)", translated.Len(), chunks)
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
			fmt.Fprintf(os.Stderr, "done (%d chars, %d chunks)", translated.Len(), chunks)
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
	fmt.Fprintf(os.Stderr, "done (%d chars, %d chunks)", translated.Len(), chunks)
	return translated.String(), nil
}

// ---- SRT text-only helpers ----

func batchSRTBlocks(blocks []srt.Block, limit int) [][]srt.Block {
	var batches [][]srt.Block
	var current []srt.Block
	currentLen := 0
	for _, block := range blocks {
		if len(block.Text) > limit {
			batches = append(batches, []srt.Block{block})
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

func buildTextInput(blocks []srt.Block) string {
	var sb strings.Builder
	for i, block := range blocks {
		fmt.Fprintf(&sb, "[%d] %s\n", i+1, block.Text)
	}
	return sb.String()
}

// MarkerIssue describes a single alignment problem in the LLM response.
type MarkerIssue struct {
	Type    string // "missing", "out_of_order", or "placeholder"
	Marker  int
	Details string
}

// ParseResult holds extracted texts (aligned by marker number) and any issues found.
type ParseResult struct {
	Texts  []string
	Issues []MarkerIssue
}

// TranslationWarnings accumulates alignment warnings across batches.
type TranslationWarnings struct {
	Missing     int
	Reordered   int
	Placeholder int
	Details     []string
}

func (w *TranslationWarnings) add(issues []MarkerIssue) {
	for _, issue := range issues {
		switch issue.Type {
		case "missing":
			w.Missing++
		case "out_of_order":
			w.Reordered++
		case "placeholder":
			w.Placeholder++
		}
		w.Details = append(w.Details, issue.Details)
	}
}

func (w *TranslationWarnings) summary() string {
	total := w.Missing + w.Reordered + w.Placeholder
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("Translation completed with %d warnings (%d missing, %d reordered, %d placeholder)", total, w.Missing, w.Reordered, w.Placeholder)
}

var textMarkerRe = regexp.MustCompile(`(?m)^\[(\d+)\]\s*`)

var placeholderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^这里是第\d+条内容[。.]?$`),
	regexp.MustCompile(`^第\d+条内容[。.]?$`),
	regexp.MustCompile(`^这是第\d+条[。.]?$`),
	regexp.MustCompile(`^内容\d+[。.]?$`),
	regexp.MustCompile(`^条目\d+[。.]?$`),
	regexp.MustCompile(`^这里是第\d+条[。.]?$`),
	regexp.MustCompile(`^\[?\d+\]?\s*内容[。.]?$`),
}

func isPlaceholder(text string) bool {
	for _, p := range placeholderPatterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

// parseTextOutput extracts translated texts from LLM response by matching [N] marker
// numbers (not position). Detects missing and out-of-order markers for retry.
func parseTextOutput(output string, expectedCount int) (*ParseResult, error) {
	matches := textMarkerRe.FindAllStringSubmatchIndex(output, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no translated entries found in response")
	}

	type markerInfo struct {
		number    int
		textStart int
	}

	markers := make([]markerInfo, 0, len(matches))
	for _, m := range matches {
		num, err := strconv.Atoi(output[m[2]:m[3]])
		if err != nil {
			continue
		}
		markers = append(markers, markerInfo{number: num, textStart: m[1]})
	}

	texts := make([]string, expectedCount)
	seen := make(map[int]bool)
	var issues []MarkerIssue

	for i, mk := range markers {
		seen[mk.number] = true

		// Detect out-of-order markers
		expectedNum := i + 1
		if mk.number != expectedNum {
			issues = append(issues, MarkerIssue{
				Type:    "out_of_order",
				Marker:  mk.number,
				Details: fmt.Sprintf("position %d: expected marker [%d] but found [%d]", i+1, expectedNum, mk.number),
			})
		}

		// Text from this marker to the next marker (or end of output)
		textEnd := len(output)
		if i+1 < len(markers) {
			textEnd = matches[i+1][0]
		}

		idx := mk.number - 1
		if idx >= 0 && idx < expectedCount {
			texts[idx] = strings.TrimSpace(output[mk.textStart:textEnd])
		}
	}

	// Detect missing markers
	for n := 1; n <= expectedCount; n++ {
		if !seen[n] {
			issues = append(issues, MarkerIssue{
				Type:    "missing",
				Marker:  n,
				Details: fmt.Sprintf("marker [%d] is missing from response", n),
			})
		}
	}

	// Detect placeholder text
	for i, text := range texts {
		if text != "" && isPlaceholder(text) {
			issues = append(issues, MarkerIssue{
				Type:    "placeholder",
				Marker:  i + 1,
				Details: fmt.Sprintf("marker [%d] contains placeholder text: %q", i+1, text),
			})
		}
	}

	return &ParseResult{Texts: texts, Issues: issues}, nil
}

func reconstructSRT(blocks []srt.Block, texts []string) string {
	// Create new blocks with translated text
	translatedBlocks := make([]srt.Block, len(blocks))
	for i, block := range blocks {
		translatedBlocks[i] = block
		if i < len(texts) {
			translatedBlocks[i].Text = texts[i]
		}
	}
	return srt.Format(translatedBlocks)
}

// ---- Shared ----

// validateTranslatedSRT checks that the translated SRT has matching timelines.
// Returns error on timeline mismatches for cache invalidation. Callers that should
// not abort should log the error as a warning instead.
func validateTranslatedSRT(source, translated string) error {
	sourceBlocks, err := srt.Parse(source)
	if err != nil {
		return fmt.Errorf("source subtitle is invalid: %w", err)
	}
	translatedBlocks, err := srt.Parse(translated)
	if err != nil {
		return fmt.Errorf("translated subtitle is invalid: %w", err)
	}

	sourceTimelines := make(map[string]bool, len(sourceBlocks))
	for _, b := range sourceBlocks {
		sourceTimelines[formatTimeline(b.Start, b.End)] = true
	}

	for _, b := range translatedBlocks {
		tl := formatTimeline(b.Start, b.End)
		if !sourceTimelines[tl] {
			return fmt.Errorf("translated timeline %q not found in source", tl)
		}
	}

	if len(sourceBlocks) != len(translatedBlocks) {
		fmt.Fprintf(os.Stderr, "Block count differs (source=%d translated=%d), continuing\n", len(sourceBlocks), len(translatedBlocks))
	}

	return nil
}

func formatTimeline(start, end time.Duration) string {
	return formatTime(start) + " --> " + formatTime(end)
}

func formatTime(d time.Duration) string {
	ms := d.Milliseconds()
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, millis)
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
