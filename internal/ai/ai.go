// Package ai turns natural-language requests into a single ffmpeg command by
// asking a large language model. It speaks to several providers (OpenAI,
// Anthropic, Gemini, Ollama and OpenRouter) over plain net/http with no SDKs,
// and parses the model's reply into a structured, reviewable Plan.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Provider is a chat-completion backend capable of turning a system+user
// prompt into a single text completion.
type Provider interface {
	// Name returns a short human label for the provider (used in messages).
	Name() string
	// Complete sends the system and user prompts and returns the raw model text.
	Complete(ctx context.Context, system, user string) (string, error)
}

// Config selects and configures a provider. Empty fields fall back to sensible
// per-provider defaults inside New.
type Config struct {
	Provider string // openai | anthropic | gemini | ollama | openrouter
	Model    string
	APIKey   string
	BaseURL  string
}

// Plan is the structured result the model must return: the ffmpeg arguments to
// run (excluding the leading "ffmpeg"), a short explanation and any warnings.
type Plan struct {
	FFmpegArgs  []string `json:"ffmpeg_args"`
	Explanation string   `json:"explanation"`
	Warnings    []string `json:"warnings"`
	Dangerous   bool     `json:"dangerous"`
}

// defaultModel returns the recommended default model for each provider.
func defaultModel(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-3-5-sonnet-latest"
	case "gemini":
		return "gemini-1.5-flash"
	case "openrouter":
		return "openai/gpt-4o-mini"
	case "ollama":
		return "llama3.1"
	default: // openai
		return "gpt-4o-mini"
	}
}

// firstEnv returns the first non-empty environment variable among names.
func firstEnv(names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(os.Getenv(n)); v != "" {
			return v
		}
	}
	return ""
}

// LoadConfig reads provider selection and credentials from the environment.
// FFGO_AI_PROVIDER selects the provider (default openai); FFGO_AI_MODEL and
// FFGO_AI_BASE_URL override the model and endpoint; the API key is taken from
// the provider's conventional variable.
func LoadConfig() (Config, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("FFGO_AI_PROVIDER")))
	if provider == "" {
		provider = "openai"
	}

	cfg := Config{
		Provider: provider,
		Model:    strings.TrimSpace(os.Getenv("FFGO_AI_MODEL")),
		BaseURL:  strings.TrimSpace(os.Getenv("FFGO_AI_BASE_URL")),
	}

	switch provider {
	case "openai", "anthropic", "gemini", "openrouter", "ollama":
		cfg.APIKey = ResolveAPIKey(provider)
	default:
		return cfg, fmt.Errorf("unknown AI provider %q (use openai, anthropic, gemini, ollama or openrouter)", provider)
	}

	return cfg, nil
}

// ResolveAPIKey returns the API key for a provider from its conventional
// environment variable. It returns "" for providers that need none (ollama)
// or that are unknown. Use it when changing the provider after LoadConfig so
// the key is re-resolved for the new provider.
func ResolveAPIKey(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return firstEnv("OPENAI_API_KEY")
	case "anthropic":
		return firstEnv("ANTHROPIC_API_KEY")
	case "gemini":
		return firstEnv("GEMINI_API_KEY", "GOOGLE_API_KEY")
	case "openrouter":
		return firstEnv("OPENROUTER_API_KEY")
	default:
		return ""
	}
}

// New builds a Provider from cfg, filling in the default model and validating
// that a required API key is present (all providers except ollama need one).
func New(cfg Config) (Provider, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel(provider)
	}

	needKey := func(env string) error {
		if strings.TrimSpace(cfg.APIKey) == "" {
			return fmt.Errorf("no API key for %s — set %s (or choose a different FFGO_AI_PROVIDER)", provider, env)
		}
		return nil
	}

	switch provider {
	case "openai":
		if err := needKey("OPENAI_API_KEY"); err != nil {
			return nil, err
		}
		return &openaiProvider{model: model, apiKey: cfg.APIKey, baseURL: cfg.BaseURL, label: "OpenAI", jsonMode: true}, nil
	case "openrouter":
		if err := needKey("OPENROUTER_API_KEY"); err != nil {
			return nil, err
		}
		base := cfg.BaseURL
		if base == "" {
			base = "https://openrouter.ai/api/v1"
		}
		return &openaiProvider{model: model, apiKey: cfg.APIKey, baseURL: base, label: "OpenRouter", jsonMode: true}, nil
	case "anthropic":
		if err := needKey("ANTHROPIC_API_KEY"); err != nil {
			return nil, err
		}
		return &anthropicProvider{model: model, apiKey: cfg.APIKey, baseURL: cfg.BaseURL}, nil
	case "gemini":
		if err := needKey("GEMINI_API_KEY or GOOGLE_API_KEY"); err != nil {
			return nil, err
		}
		return &geminiProvider{model: model, apiKey: cfg.APIKey, baseURL: cfg.BaseURL}, nil
	case "ollama":
		return &ollamaProvider{model: model, baseURL: cfg.BaseURL}, nil
	default:
		return nil, fmt.Errorf("unknown AI provider %q", provider)
	}
}

// ParsePlan extracts the first JSON object from the model text — tolerating
// Markdown code fences and surrounding prose — and unmarshals it into a Plan.
func ParsePlan(modelText string) (Plan, error) {
	raw := extractJSON(modelText)
	if raw == "" {
		return Plan{}, fmt.Errorf("no JSON object found in model response")
	}

	var p Plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Plan{}, fmt.Errorf("could not parse model response as JSON: %w", err)
	}

	// Guard against the model prepending a literal "ffmpeg" token.
	if len(p.FFmpegArgs) > 0 && strings.EqualFold(strings.TrimSpace(p.FFmpegArgs[0]), "ffmpeg") {
		p.FFmpegArgs = p.FFmpegArgs[1:]
	}
	if len(p.FFmpegArgs) == 0 {
		return Plan{}, fmt.Errorf("model returned no ffmpeg arguments")
	}
	return p, nil
}

// extractJSON returns the first balanced { ... } JSON object found in s, after
// stripping any Markdown code fences. It respects string literals and escapes
// so that braces inside quoted values do not confuse the scanner.
func extractJSON(s string) string {
	s = stripFences(s)

	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// stripFences removes surrounding ``` / ```json Markdown fences if present.
func stripFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return s
	}
	// Drop the opening fence line (```lang) and any trailing closing fence.
	if nl := strings.IndexByte(trimmed, '\n'); nl >= 0 {
		trimmed = trimmed[nl+1:]
	}
	if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return trimmed
}
