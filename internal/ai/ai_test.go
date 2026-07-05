package ai

import (
	"testing"
)

func TestParsePlan(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantLen int
		wantErr bool
		want0   string
	}{
		{"fenced", "```json\n{\"ffmpeg_args\":[\"-i\",\"a.mp4\",\"b.mp4\"],\"explanation\":\"x\"}\n```", 3, false, "-i"},
		{"prose", "Sure, here you go:\n{\"ffmpeg_args\":[\"-i\",\"a.mp4\"]}\nHope that helps!", 2, false, "-i"},
		{"strips-ffmpeg", "{\"ffmpeg_args\":[\"ffmpeg\",\"-i\",\"a.mp4\"]}", 2, false, "-i"},
		{"braces-in-string", "{\"ffmpeg_args\":[\"-vf\",\"drawtext=text={hi}\"],\"explanation\":\"\"}", 2, false, "-vf"},
		{"no-json", "I cannot help with that.", 0, true, ""},
		{"empty-args", "{\"ffmpeg_args\":[]}", 0, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := ParsePlan(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", p)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(p.FFmpegArgs) != c.wantLen {
				t.Fatalf("args len = %d, want %d (%v)", len(p.FFmpegArgs), c.wantLen, p.FFmpegArgs)
			}
			if p.FFmpegArgs[0] != c.want0 {
				t.Errorf("args[0] = %q, want %q", p.FFmpegArgs[0], c.want0)
			}
		})
	}
}

func TestLoadConfigDefault(t *testing.T) {
	t.Setenv("FFGO_AI_PROVIDER", "")
	t.Setenv("FFGO_AI_MODEL", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("default provider = %q, want openai", cfg.Provider)
	}

	t.Setenv("FFGO_AI_PROVIDER", "ollama")
	if cfg, _ := LoadConfig(); cfg.Provider != "ollama" {
		t.Errorf("provider = %q, want ollama", cfg.Provider)
	}

	t.Setenv("FFGO_AI_PROVIDER", "bogus")
	if _, err := LoadConfig(); err == nil {
		t.Error("unknown provider should error")
	}
}

func TestNewRequiresKey(t *testing.T) {
	if _, err := New(Config{Provider: "openai"}); err == nil {
		t.Error("openai without key should error")
	}
	if _, err := New(Config{Provider: "openai", APIKey: "sk-test"}); err != nil {
		t.Errorf("openai with key should succeed: %v", err)
	}
	// Ollama is local and needs no key.
	if _, err := New(Config{Provider: "ollama"}); err != nil {
		t.Errorf("ollama should not require a key: %v", err)
	}
	if _, err := New(Config{Provider: "anthropic"}); err == nil {
		t.Error("anthropic without key should error")
	}
	p, err := New(Config{Provider: "ollama"})
	if err != nil || p.Name() == "" {
		t.Errorf("provider should have a name")
	}
}
