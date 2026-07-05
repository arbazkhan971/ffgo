package explain

import (
	"strings"
	"testing"
)

func TestExplain(t *testing.T) {
	args := []string{
		"-i", "in.mp4",
		"-vf", "scale=1280:-1",
		"-crf", "23",
		"-c:a", "copy",
		"-an",
		"-ss", "5",
		"-movflags", "+faststart",
		"out.mp4",
	}
	segs := Explain(args)
	if len(segs) == 0 {
		t.Fatal("no segments returned")
	}

	// Build a lookup from the leading token of each segment.
	all := strings.ToLower(joinDetails(segs))
	checks := map[string]string{
		"input":  "in.mp4",
		"scale":  "scale",
		"crf":    "quality",
		"copy":   "copy",
		"audio":  "audio",
		"start":  "seek",
		"stream": "faststart",
	}
	for name, want := range checks {
		if !strings.Contains(all, want) {
			t.Errorf("explanation missing %q keyword for %s; got: %s", want, name, all)
		}
	}

	// The last segment should describe the output file.
	last := segs[len(segs)-1]
	if !strings.Contains(last.Token, "out.mp4") || !strings.Contains(strings.ToLower(last.Detail), "output") {
		t.Errorf("last segment should be the output file, got %+v", last)
	}
}

func TestExplainUnknownFlag(t *testing.T) {
	segs := Explain([]string{"-someweirdflag", "42", "out.mkv"})
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	// Should not panic and should still identify the output file.
	last := segs[len(segs)-1]
	if !strings.Contains(last.Token, "out.mkv") {
		t.Errorf("expected output file last, got %+v", last)
	}
}

func joinDetails(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Token)
		b.WriteByte(' ')
		b.WriteString(s.Detail)
		b.WriteByte('\n')
	}
	return b.String()
}

func BenchmarkExplain(b *testing.B) {
	args := []string{"-i", "in.mp4", "-vf", "scale=1280:-1", "-crf", "23",
		"-preset", "slow", "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", "out.mp4"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Explain(args)
	}
}
