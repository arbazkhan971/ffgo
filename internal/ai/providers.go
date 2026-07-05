package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpClient is shared by every provider; a 60s timeout keeps a hung endpoint
// from blocking the CLI forever.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// postJSON marshals body, POSTs it to url with the given headers and returns
// the response body, erroring with a status + snippet on any non-2xx reply.
func postJSON(ctx context.Context, url string, headers map[string]string, body any) ([]byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, snippet(data))
	}
	return data, nil
}

// snippet trims a response body to a short single-line excerpt for errors.
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	s = strings.Join(strings.Fields(s), " ")
	const max = 300
	if len(s) > max {
		s = s[:max] + "…"
	}
	if s == "" {
		s = "(empty response)"
	}
	return s
}

// ---------------------------------------------------------------------------
// OpenAI-compatible (OpenAI + OpenRouter)
// ---------------------------------------------------------------------------

type openaiProvider struct {
	model    string
	apiKey   string
	baseURL  string
	label    string
	jsonMode bool
}

func (p *openaiProvider) Name() string { return p.label }

func (p *openaiProvider) Complete(ctx context.Context, system, user string) (string, error) {
	base := p.baseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(base, "/") + "/chat/completions"

	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.2,
	}
	if p.jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	headers := map[string]string{"Authorization": "Bearer " + p.apiKey}
	data, err := postJSON(ctx, url, headers, body)
	if err != nil {
		return "", err
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("model returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}

// ---------------------------------------------------------------------------
// Anthropic
// ---------------------------------------------------------------------------

type anthropicProvider struct {
	model   string
	apiKey  string
	baseURL string
}

func (p *anthropicProvider) Name() string { return "Anthropic" }

func (p *anthropicProvider) Complete(ctx context.Context, system, user string) (string, error) {
	base := p.baseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	url := strings.TrimRight(base, "/") + "/v1/messages"

	body := map[string]any{
		"model":      p.model,
		"max_tokens": 1024,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	}
	headers := map[string]string{
		"x-api-key":         p.apiKey,
		"anthropic-version": "2023-06-01",
	}
	data, err := postJSON(ctx, url, headers, body)
	if err != nil {
		return "", err
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("model returned no text content")
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// Google Gemini
// ---------------------------------------------------------------------------

type geminiProvider struct {
	model   string
	apiKey  string
	baseURL string
}

func (p *geminiProvider) Name() string { return "Gemini" }

func (p *geminiProvider) Complete(ctx context.Context, system, user string) (string, error) {
	base := p.baseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimRight(base, "/"), p.model, p.apiKey)

	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": user}}},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": system}},
		},
		"generationConfig": map[string]any{
			"temperature":      0.2,
			"responseMimeType": "application/json",
		},
	}
	data, err := postJSON(ctx, url, nil, body)
	if err != nil {
		return "", err
	}

	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("model returned no candidates")
	}
	var sb strings.Builder
	for _, part := range out.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("model returned empty content")
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// Ollama (local)
// ---------------------------------------------------------------------------

type ollamaProvider struct {
	model   string
	baseURL string
}

func (p *ollamaProvider) Name() string { return "Ollama" }

func (p *ollamaProvider) Complete(ctx context.Context, system, user string) (string, error) {
	base := p.baseURL
	if base == "" {
		base = "http://localhost:11434"
	}
	url := strings.TrimRight(base, "/") + "/api/chat"

	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream": false,
	}
	data, err := postJSON(ctx, url, nil, body)
	if err != nil {
		return "", err
	}

	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if strings.TrimSpace(out.Message.Content) == "" {
		return "", fmt.Errorf("model returned empty content")
	}
	return out.Message.Content, nil
}
